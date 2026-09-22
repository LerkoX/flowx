package dag

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/executor"
	"github.com/LerkoX/flowx/logger"
	"github.com/LerkoX/flowx/metadata"
)

// Run 执行流水线
func (p *WorkflowImpl) Run(ctx context.Context) error {
	p.mu.Lock()
	if p.status == core.StatusRunning {
		p.mu.Unlock()
		return fmt.Errorf("%w: workflow is already running", core.ErrInvalidState)
	}
	// 重新创建 doneChan，支持多次 Run
	if p.doneChan == nil {
		p.doneChan = make(chan struct{})
	}
	p.cleanupOnce = sync.Once{}
	ctx, cancel := context.WithCancel(ctx)
	p.cancelFunc = cancel
	p.status = core.StatusRunning
	p.mu.Unlock()

	defer func() {
		p.cleanupOnce.Do(func() {
			p.mu.Lock()
			if p.cancelFunc != nil {
				p.cancelFunc()
				p.cancelFunc = nil
			}
			p.mu.Unlock()

			// 清理所有executor。注意必须用独立的 context：上方 cancelFunc
			// 已取消 ctx，若沿用 canceled ctx 发 Docker API（ContainerStop/
			// ContainerRemove）会立即失败，导致容器残留泄漏。
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cleanupCancel()
			p.cleanupExecutors(cleanupCtx)

			p.mu.Lock()
			close(p.doneChan)
			p.doneChan = nil
			p.mu.Unlock()
		})
	}()

	// 通知流水线开始
	p.NotifyEvent(WorkflowStart)

	// 创建求值上下文
	evalCtx := NewEvaluationContext().WithWorkflow(p)

	// 如果有元数据存储，加载数据到求值上下文
	if p.metadataStore != nil {
		params := make(map[string]any)
		if inConfigStore, ok := p.metadataStore.(*metadata.InConfigMetadataStore); ok {
			for key := range inConfigStore.GetAll() {
				if val, err := inConfigStore.Get(ctx, key); err == nil {
					params[key] = val
				}
			}
		}
		if len(params) > 0 {
			evalCtx = evalCtx.WithParams(params)
		}
	}

	// 获取 DGAGraph，如果非 DGAGraph 实现则降级到原始 Traversal
	dgaGraph, ok := p.graph.(*DGAGraph)
	if !ok {
		err := p.graph.Traversal(ctx, evalCtx, p.makeTraversalFn(ctx))
		if err != nil {
			p.mu.Lock()
			p.status = core.StatusFailed
			p.mu.Unlock()
		} else {
			p.mu.Lock()
			p.status = core.StatusSuccess
			p.mu.Unlock()
		}
		p.NotifyEvent(WorkflowFinish)
		return err
	}

	// 逐层 BFS 执行，同时支持有环图和无环图
	startLevel := p.restoreTraversalState()
	if err := p.runLevelByLevel(ctx, dgaGraph, evalCtx, startLevel); err != nil {
		p.mu.Lock()
		p.status = core.StatusFailed
		p.mu.Unlock()
		p.NotifyEvent(WorkflowFinish)
		return err
	}

	// 通知流水线完成
	p.mu.Lock()
	p.status = core.StatusSuccess
	p.mu.Unlock()
	p.NotifyEvent(WorkflowFinish)
	return nil
}

// runLevelByLevel 逐层执行 BFS 遍历，支持暂停/恢复和循环图
// 无环图：执行完所有层级后直接返回
// 有环图：每轮迭代执行循环体并评估回边条件，满足则重置循环节点继续迭代；
// 循环出口下游节点（回边 source 之后的非循环体节点）推迟到循环退出后再执行
func (p *WorkflowImpl) runLevelByLevel(ctx context.Context, dgaGraph *DGAGraph, evalCtx EvaluationContext, startLevel int) error {
	iteration := 0
	p.mu.RLock()
	maxIter := p.maxLoopIter
	p.mu.RUnlock()

	for {
		evalCtx = evalCtx.WithIteration(iteration)

		backEdges := dgaGraph.BackEdges()

		// 有环图：迭代期间排除循环出口下游节点，待循环退出后统一执行
		var exclude map[string]bool
		if len(backEdges) > 0 {
			exclude = dgaGraph.LoopExitNodeSet(backEdges)
		}

		if err := p.executeLevels(ctx, dgaGraph, evalCtx, startLevel, exclude); err != nil {
			return err
		}

		if len(backEdges) == 0 {
			return nil // 无环图：直接返回
		}

		// 循环检测：评估回边条件
		shouldContinue := false
		var activeLoopNodes map[string]bool

		for _, backEdge := range backEdges {
			evalCtxWithIter := evalCtx.WithIteration(iteration + 1) // 下一次迭代的 iteration 值
			result, err := backEdge.Evaluate(evalCtxWithIter)
			if err != nil {
				return fmt.Errorf("failed to evaluate back-edge condition %s->%s: %w",
					backEdge.Source().Id(), backEdge.Target().Id(), err)
			}
			if result {
				shouldContinue = true
				// 合并所有活跃回边的循环节点集合
				loopNodes := dgaGraph.LoopNodeSet(backEdge)
				if activeLoopNodes == nil {
					activeLoopNodes = make(map[string]bool)
				}
				for k := range loopNodes {
					activeLoopNodes[k] = true
				}
			}
		}

		if !shouldContinue {
			// 循环结束：执行此前推迟的循环出口下游节点（其余节点已终结会被跳过）
			if len(exclude) > 0 {
				return p.executeLevels(ctx, dgaGraph, evalCtx, 0, nil)
			}
			return nil
		}

		iteration++
		if iteration >= maxIter {
			return fmt.Errorf("loop exceeded maximum iterations (%d)", maxIter)
		}

		// 重置循环节点的运行时状态，使其可以重新执行
		p.resetLoopNodes(dgaGraph, activeLoopNodes)
		startLevel = 0 // 从头开始遍历
	}
}

// executeLevels 逐层执行 BFS 层级计划
// exclude 非空时从层级计划中排除指定节点（用于循环迭代期间推迟循环出口下游节点）
func (p *WorkflowImpl) executeLevels(ctx context.Context, dgaGraph *DGAGraph, evalCtx EvaluationContext, startLevel int, exclude map[string]bool) error {
	// 计算初始层级
	levels := filterLevels(dgaGraph.TraversalSteps(evalCtx), exclude)
	if len(levels) == 0 {
		return nil
	}

	var firstErr error
	levelIdx := startLevel

	for levelIdx < len(levels) {
		// 检查暂停信号
		p.pauseMu.Lock()
		for p.status == core.StatusPaused {
			// 保存当前层级并进入暂停状态
			p.mu.Lock()
			p.currentLevel = levelIdx
			p.mu.Unlock()
			p.NotifyEvent(WorkflowPaused)

			// 等待恢复信号
			p.pauseCond.Wait()

			// 恢复运行
			p.mu.Lock()
			p.status = core.StatusRunning
			p.mu.Unlock()
			p.NotifyEvent(WorkflowResumed)

			// 重新计算层级（图可能已被修改）
			levels = filterLevels(dgaGraph.TraversalSteps(evalCtx), exclude)
			if levelIdx >= len(levels) {
				p.pauseMu.Unlock()
				return nil
			}
		}
		p.pauseMu.Unlock()

		// 检查 context 是否已取消
		select {
		case <-ctx.Done():
			p.mu.Lock()
			p.status = core.StatusCancelled
			p.mu.Unlock()
			return ctx.Err()
		default:
		}

		// 执行当前层级的所有节点
		level := levels[levelIdx]
		var wg sync.WaitGroup
		var errMu sync.Mutex

		for _, nodeID := range level {
			node, exists := dgaGraph.GetNode(nodeID)
			if !exists {
				continue
			}

			wg.Add(1)
			go func(n Node) {
				defer wg.Done()
				if err := p.executeNodeWithLifecycle(ctx, n); err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
				}
			}(node)
		}
		wg.Wait()

		if firstErr != nil {
			return firstErr
		}

		levelIdx++

		// 当前层执行完毕后，重新计算后续层级（确保条件边能获取到最新的 metadata）
		if levelIdx < len(levels) {
			levels = filterLevels(dgaGraph.TraversalSteps(evalCtx), exclude)
		}
	}

	return nil
}

// filterLevels 从层级计划中排除指定节点（exclude 为空时原样返回）
func filterLevels(levels [][]string, exclude map[string]bool) [][]string {
	if len(exclude) == 0 {
		return levels
	}
	filtered := make([][]string, 0, len(levels))
	for _, level := range levels {
		kept := make([]string, 0, len(level))
		for _, id := range level {
			if !exclude[id] {
				kept = append(kept, id)
			}
		}
		if len(kept) > 0 {
			filtered = append(filtered, kept)
		}
	}
	return filtered
}

func (p *WorkflowImpl) resetLoopNodes(dgaGraph *DGAGraph, loopNodes map[string]bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for nodeID := range loopNodes {
		node, exists := dgaGraph.GetNode(nodeID)
		if !exists {
			continue
		}
		// 重置节点运行时状态为 nil，使 shouldSkipNode 不再跳过
		node.SetRuntimeStatus(nil)

		// 清理节点提取的 metadata（避免旧数据影响后续迭代）
		if p.metadata != nil {
			prefix := nodeID + "."
			for k := range p.metadata {
				if len(k) > len(prefix) && k[:len(prefix)] == prefix {
					delete(p.metadata, k)
				}
			}
		}
	}
}

// makeTraversalFn 创建节点执行函数（兼容旧的 Traversal 调用方式）
func (p *WorkflowImpl) makeTraversalFn(ctx context.Context) TraversalFn {
	return func(ctx context.Context, node Node) error {
		return p.executeNodeWithLifecycle(ctx, node)
	}
}

// executeNodeWithLifecycle 执行节点的完整生命周期（跳过检查→通知→执行→通知）
func (p *WorkflowImpl) executeNodeWithLifecycle(ctx context.Context, node Node) error {
	// 检查 context 是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 检查是否应该跳过此节点
	if p.shouldSkipNode(node) {
		fmt.Printf("Skipping node %s (status: %s)\n", node.Id(), node.GetRuntimeStatus().Status)
		p.NotifyEventForNode(WorkflowNodeFinish, node)
		return nil
	}

	// 设置当前节点
	p.mu.Lock()
	p.currentNode = node
	p.mu.Unlock()

	// 通知节点开始
	p.NotifyEventForNode(WorkflowNodeStart, node)
	fmt.Printf("Executing node: %s\n", node.Id())

	// 获取节点的executor配置
	executorName := node.GetExecutor()
	if executorName == "" {
		fmt.Printf("Node %s has no executor configured, skipping\n", node.Id())
		p.NotifyEventForNode(WorkflowNodeFinish, node)
		// 清理当前节点
		p.mu.Lock()
		p.currentNode = nil
		p.mu.Unlock()
		return nil
	}

	// 获取或创建executor
	exec, err := p.getOrCreateExecutor(ctx, executorName)
	if err != nil {
		fmt.Printf("Failed to get executor for node %s: %v\n", node.Id(), err)
		// executor 获取/准备失败（如镜像拉取失败、daemon 不可达）也是节点失败，
		// 必须触发 WorkflowNodeFailed——否则上游只能看到 node-start 没有收尾事件，
		// 节点状态会永远停留在 running
		p.NotifyEventForNode(WorkflowNodeFailed, node)
		// 清理当前节点
		p.mu.Lock()
		p.currentNode = nil
		p.mu.Unlock()
		return fmt.Errorf("failed to get executor for node %s: %w", node.Id(), err)
	}

	// 执行节点
	if err := p.executeNode(ctx, node, exec); err != nil {
		fmt.Printf("Node %s execution failed: %v\n", node.Id(), err)
		// 节点执行失败时触发 WorkflowNodeFailed 事件
		p.NotifyEventForNode(WorkflowNodeFailed, node)
		// 清理当前节点
		p.mu.Lock()
		p.currentNode = nil
		p.mu.Unlock()
		return err
	}

	// 通知节点完成
	p.NotifyEventForNode(WorkflowNodeFinish, node)

	// 清理当前节点
	p.mu.Lock()
	p.currentNode = nil
	p.mu.Unlock()

	return nil
}

// executeNode 执行单个节点
func (p *WorkflowImpl) executeNode(ctx context.Context, node Node, exec executor.Executor) error {
	steps := node.GetSteps()
	if len(steps) == 0 {
		fmt.Printf("Node %s has no steps to execute\n", node.Id())
		return nil
	}

	// 1. 初始化节点运行时状态
	runtimeStatus := p.initializeNodeRuntimeStatus(node, exec)
	node.SetRuntimeStatus(runtimeStatus)

	// 2. 设置通道和启动 executor
	commandChan, resultChan, inputChan := p.setupExecutorChannels(ctx, exec, steps)

	// 将 inputChan 保存到 RuntimeStatus，供外部交互使用
	runtimeStatus = node.GetRuntimeStatus()
	if runtimeStatus != nil {
		runtimeStatus.InputChan = inputChan
		node.SetRuntimeStatus(runtimeStatus)
	}

	// 3. 上游绑定引用静态校验：{{ Upstream.field }} 中上游未产出的字段在此直接失败。
	// pongo2 非 strict 模式会把缺失字段渲染成空串（不报错），错误最终落到下游节点的
	// "缺参"上，掩盖真正的上游问题（exec 364：docker exec 流被截断 → KSampler 无输出）。
	if err := p.validateNodeBindings(node); err != nil {
		fmt.Printf("Node %s binding validation failed: %v\n", node.Id(), err)
		if rs := node.GetRuntimeStatus(); rs != nil {
			p.updateNodeFinalStatus(rs, err)
			node.SetRuntimeStatus(rs)
		}
		return err
	}

	// 4. 发送所有步骤命令
	go p.sendCommands(ctx, node, commandChan, steps)

	// 5. 等待并处理所有结果
	fullOutput, lastErr := p.waitForResults(ctx, node, exec, resultChan, steps)

	// 6. 从完整输出中提取元数据
	if lastErr == nil {
		// 创建一个虚拟 executor.StepResult 用于提取
		stepResult := &executor.StepResult{
			StepName:  node.Id(),
			Output:    fullOutput,
			StartTime: time.Now(),
		}
		if err := p.extractOutput(ctx, node, stepResult, fullOutput); err != nil {
			fmt.Printf("Failed to extract output from node %s: %v\n", node.Id(), err)
		}
	}

	// 7. 更新节点最终状态
	if runtimeStatus = node.GetRuntimeStatus(); runtimeStatus != nil {
		p.updateNodeFinalStatus(runtimeStatus, lastErr)
		node.SetRuntimeStatus(runtimeStatus)
	}

	// 8. 等待 executor Transfer 完全退出后再关闭 inputChan
	//    Transfer 会在 commandChan 关闭后退出，sendCommands 负责关闭 commandChan。
	//    这里 drain resultChan 直到关闭，以确认 Transfer 已结束。
	go func() {
		for range resultChan {
		}
	}()
	select {
	case <-resultChan:
	case <-time.After(5 * time.Second):
		// 超时，强制继续
	}

	close(inputChan)

	return lastErr
}

// initializeNodeRuntimeStatus 初始化节点运行时状态
func (p *WorkflowImpl) initializeNodeRuntimeStatus(node Node, exec executor.Executor) *core.NodeRuntimeStatus {
	node.EnsureIds()
	runtimeStatus := node.GetRuntimeStatus()
	if runtimeStatus == nil {
		runtimeStatus = &core.NodeRuntimeStatus{
			Id:    core.NewUUID(),
			Steps: []core.StepRuntimeStatus{},
		}
	}

	// 更新节点状态
	runtimeStatus.Status = core.StatusRunning
	runtimeStatus.StartTime = time.Now().Format(time.RFC3339)

	// 收集 executor.Executor 信息
	if infoProvider, ok := exec.(executor.ExecutorInfoProvider); ok {
		runtimeStatus.Executor = &core.ExecutorRuntimeInfo{
			Type:       infoProvider.GetType(),
			InstanceId: infoProvider.GetInstanceId(),
			Status:     executor.ExecutorStatusRunning,
			Info:       infoProvider.GetRuntimeInfo(),
		}
	}

	return runtimeStatus
}

// setupExecutorChannels 设置通道并启动 executor
func (p *WorkflowImpl) setupExecutorChannels(ctx context.Context, exec executor.Executor, steps []core.Step) (chan any, chan any, chan []byte) {
	commandChan := make(chan any, len(steps))
	resultChan := make(chan any, len(steps)*10)
	inputChan := make(chan []byte, 100) // 输入通道，缓冲100条消息

	// 启动 executor 的 Transfer goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				select {
				case resultChan <- fmt.Errorf("executor panic: %v", r):
				default:
				}
			}
		}()
		exec.Transfer(ctx, resultChan, commandChan, inputChan)
	}()

	return commandChan, resultChan, inputChan
}

// sendCommands 发送所有步骤命令
func (p *WorkflowImpl) sendCommands(ctx context.Context, node Node, commandChan chan any, steps []core.Step) {
	defer close(commandChan)
	for _, step := range steps {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 检查 step 是否已完成（运行时状态恢复）
		if p.shouldSkipStep(node, step.Name) {
			fmt.Printf("Skipping step %s (status: %s)\n", step.Name, getStepStatusString(node, step.Name))
			continue
		}

		// 节点级渲染上下文：节点 params 绑定优先于 workflow 级 Param
		nodeCtx := p.buildNodeRenderContext(node)

		// 运行时渲染 step.Run，可以引用前面节点 extract 的数据
		renderedRun, err := p.renderNodeStringWithContext(node, nodeCtx, step.Run)
		if err != nil {
			fmt.Printf("Failed to render step %s: %v, using original command\n", step.Name, err)
			renderedRun = step.Run // 渲染失败时使用原始命令
		}

		// 环境变量随命令下发，由执行器以真实进程环境变量注入
		//（不经 shell 解析，值中的引号/换行/特殊字符天然安全；
		//  不支持 env 的执行器由其在渲染后做单引号转义兑底）。
		// 运行时身份变量：节点可据此区分不同执行实例/节点，实现按执行隔离的
		// 持久化状态（如交互会话：新执行开新会话，同一执行的续跑/重入才复用）。
		env := map[string]string{
			"FLOWX_WORKFLOW_ID": p.Id(),
			"FLOWX_NODE_ID":     node.Id(),
		}
		for k, v := range nodeStringMap(node, "env") {
			rendered, rerr := p.renderNodeStringWithContext(node, nodeCtx, v)
			if rerr != nil {
				fmt.Printf("Failed to render env %s for node %s: %v, using raw value\n", k, node.Id(), rerr)
				rendered = v
			}
			env[k] = rendered // 节点显式 env 优先于内置身份变量
		}

		commandChan <- executor.CommandWrapper{
			StepName:      step.Name,
			Command:       renderedRun,
			Env:           env,
			CaptureOutput: nodeDeclaresExtract(node),
		}
	}
}

// nodeDeclaresExtract 判断节点是否声明了输出提取（extract 配置）。
// 这类节点的输出块是下游节点的数据来源，一旦丢失，下游会报"缺参"而掩盖真因
// （exec 364）；故 docker 执行器对其启用容器内 tee 兜底（落盘 + 断流后补齐尾部）。
// 判定条件与 extractOutput 一致：extract 存在且非 nil。
func nodeDeclaresExtract(node Node) bool {
	cfg := node.GetConfig()
	if cfg == nil {
		return false
	}
	extract, ok := cfg["extract"]
	return ok && extract != nil
}

// shouldSkipStep 检查步骤是否应该跳过执行
func (p *WorkflowImpl) shouldSkipStep(node Node, stepName string) bool {
	runtimeStatus := node.GetRuntimeStatus()
	if runtimeStatus == nil {
		return false
	}

	// 查找 step 的运行时状态
	for _, step := range runtimeStatus.Steps {
		if step.Name == stepName {
			// SUCCESS、FAILED、CANCELLED 状态的 step 都跳过
			switch step.Status {
			case core.StatusSuccess, core.StatusFailed, core.StatusCancelled:
				return true
			default:
				return false
			}
		}
	}
	return false
}

// getStepStatusString 辅助函数：获取 step 状态字符串（避免 nil panic）
func getStepStatusString(node Node, stepName string) string {
	runtimeStatus := node.GetRuntimeStatus()
	if runtimeStatus == nil {
		return "unknown"
	}

	for _, step := range runtimeStatus.Steps {
		if step.Name == stepName {
			return step.Status
		}
	}
	return "unknown"
}

// waitForResults 等待并处理所有结果
func (p *WorkflowImpl) waitForResults(ctx context.Context, node Node, exec executor.Executor, resultChan chan any, steps []core.Step) (string, error) {
	var lastErr error
	resultCount := 0
	// 计算实际需要执行的步骤数量（不包括已完成的步骤）
	expectedResults := 0
	for _, step := range steps {
		if !p.shouldSkipStep(node, step.Name) {
			expectedResults++
		}
	}
	// 如果没有需要执行的步骤，直接返回
	if expectedResults == 0 {
		return "", nil
	}
	var allOutput strings.Builder

	for resultCount < expectedResults {
		select {
		case <-ctx.Done():
			p.handleCancellation(nodeContext{ctx: ctx, node: node})
			return allOutput.String(), ctx.Err()
		case result, ok := <-resultChan:
			if !ok {
				return allOutput.String(), lastErr
			}

			errCount := p.handleResult(ctx, node, exec, result, resultCount, steps)
			if errCount.err != nil {
				lastErr = errCount.err
			}
			resultCount = errCount.count
			allOutput.WriteString(errCount.output)
		}
	}

	return allOutput.String(), lastErr
}

// handleCancellation 处理节点取消
func (p *WorkflowImpl) handleCancellation(ctx nodeContext) {
	runtimeStatus := ctx.node.GetRuntimeStatus()
	if runtimeStatus != nil {
		runtimeStatus.Status = core.StatusCancelled
		runtimeStatus.EndTime = time.Now().Format(time.RFC3339)
		ctx.node.SetRuntimeStatus(runtimeStatus)
	}
}

// handleResult 处理单个结果
func (p *WorkflowImpl) handleResult(ctx context.Context, node Node, _ executor.Executor, result any, resultCount int, steps []core.Step) resultHandler {
	handler := resultHandler{count: resultCount}

	switch v := result.(type) {
	case error:
		handler.err = v
		handler.count++
	case *executor.StepResult:
		if v.Error != nil {
			handler.err = v.Error
		}

		// 执行器报告输出流曾被截断（已尽力重挂/补齐）：节点可能绿灯但输出块不完整。
		// 留显式标记，供 Studio 在节点/详情上提示"输出可能不完整"。
		if v.StreamTruncated {
			p.markStreamTruncated(ctx, node, v.StepName)
		}

		// 通过步骤名称查找对应的步骤（修复索引映射错误）
		var targetStep *core.Step
		for i := range steps {
			if steps[i].Name == v.StepName {
				targetStep = &steps[i]
				break
			}
		}

		if targetStep != nil {
			p.updateStepRuntimeStatus(node, *targetStep, v)
		} else {
			fmt.Printf("Warning: executor.StepResult for '%s' not found in steps array\n", v.StepName)
		}
		handler.count++
	case []byte:
		// 实时输出 - 通过 pusher 推送
		output := string(v)
		level := logger.LevelInfo
		// 行首级别标记约定：节点输出以 [flowx:error] / [flowx:warn] / [flowx:debug]
		// 开头时按对应级别记录（剥离标记），便于前端按级别提示节点异常
		for _, m := range []struct {
			prefix string
			level  logger.Level
		}{
			{"[flowx:error]", logger.LevelError},
			{"[flowx:warn]", logger.LevelWarn},
			{"[flowx:debug]", logger.LevelDebug},
		} {
			if strings.HasPrefix(output, m.prefix) {
				level = m.level
				output = strings.TrimPrefix(strings.TrimPrefix(output, m.prefix), " ")
				break
			}
		}
		if p.pusher != nil {
			_ = p.pusher.Push(ctx, logger.Entry{
				Workflow: p.Id(),
				Node:     node.Id(),
				Level:    level,
				Message:  output,
				Output:   output,
			})
		}

		handler.output = output
	case *executor.InputRequestEvent:
		// 程序请求用户输入
		p.handleInputRequest(node, v)
	case *executor.InputReadyEvent:
		// 输入通道已就绪（可以忽略，因为我们已经创建了 InputChan）
	}

	return handler
}

// handleInputRequest 处理输入请求
// 当程序输出 {"flowx":"wait-input",...} 时被调用
func (p *WorkflowImpl) handleInputRequest(node Node, event *executor.InputRequestEvent) {
	if event == nil || event.Request == nil {
		return
	}

	runtimeStatus := node.GetRuntimeStatus()
	if runtimeStatus == nil {
		return
	}

	// 更新节点状态为 PAUSED（等待输入）
	runtimeStatus.Status = core.StatusPaused
	runtimeStatus.InputRequest = &core.InputRequestInfo{
		StepName: event.StepName,
		Prompt:   event.Request.Prompt,
		Type:     event.Request.Type,
	}
	node.SetRuntimeStatus(runtimeStatus)

	// 触发暂停事件，通知监听器处理输入请求
	p.NotifyEvent(WorkflowPaused)
}

// resultHandler 处理结果的辅助结构
type resultHandler struct {
	err    error
	count  int
	output string
}

// nodeContext 节点上下文辅助结构
type nodeContext struct {
	ctx  context.Context
	node Node
}

// updateStepRuntimeStatus 更新步骤运行时状态
func (p *WorkflowImpl) updateStepRuntimeStatus(node Node, step core.Step, result *executor.StepResult) {
	stepStatus := core.StepRuntimeStatus{
		Id:     step.Id,
		Name:   step.Name,
		Status: core.StatusSuccess,
	}

	if !result.StartTime.IsZero() {
		stepStatus.StartTime = result.StartTime.Format(time.RFC3339)
	}
	if !result.FinishTime.IsZero() {
		stepStatus.EndTime = result.FinishTime.Format(time.RFC3339)
	}
	if result.Error != nil {
		stepStatus.Status = core.StatusFailed
		stepStatus.Error = result.Error.Error()
	}

	node.SetStepRuntimeStatus(&stepStatus)
}

// updateNodeFinalStatus 更新节点最终状态
func (p *WorkflowImpl) updateNodeFinalStatus(runtimeStatus *core.NodeRuntimeStatus, err error) {
	if err != nil {
		runtimeStatus.Status = core.StatusFailed
	} else {
		runtimeStatus.Status = core.StatusSuccess
	}
	runtimeStatus.EndTime = time.Now().Format(time.RFC3339)

	// 更新 executor.Executor 状态为 DESTROYED
	if runtimeStatus.Executor != nil {
		runtimeStatus.Executor.Status = executor.ExecutorStatusDestroyed
	}
}
