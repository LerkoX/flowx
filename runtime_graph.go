package flowx

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/dag"
	"github.com/LerkoX/flowx/template"
	"github.com/tetrafolium/mermaid-check/ast"
	"github.com/tetrafolium/mermaid-check/parser"
)

// buildGraph 构建图结构
func (r *RuntimeImpl) buildGraph(config *core.PipelineConfig) dag.Graph {
	graph := dag.NewDGAGraph()

	// 创建节点
	nodeMap := make(map[string]dag.Node)
	for nodeName, nodeConfig := range config.Nodes {
		// nodeConfig 是结构体副本，但 Steps 切片与 config.Nodes 共享底层数组，
		// 先深拷贝再分配 ID，避免污染存储的配置（否则 UpdateConfig 比对时
		// 旧配置步骤已带运行时 ID，与新解析的配置不相等，误判为节点被修改）
		stepsCopy := make([]core.Step, len(nodeConfig.Steps))
		copy(stepsCopy, nodeConfig.Steps)
		nodeConfig.Steps = stepsCopy

		// 初始状态：如果有 runtime 则用 runtime 的 status，否则用 core.StatusUnknown
		initialStatus := core.StatusUnknown
		if nodeConfig.Runtime != nil && nodeConfig.Runtime.Status != "" {
			initialStatus = nodeConfig.Runtime.Status
		}

		// 确保步骤有ID
		for i := range nodeConfig.Steps {
			if nodeConfig.Steps[i].Id == "" {
				nodeConfig.Steps[i].Id = core.NewUUID()
			}
		}

		// 构建节点配置，包含 extract 配置
		nodeConfigMap := make(map[string]any)
		for k, v := range nodeConfig.Config {
			nodeConfigMap[k] = v
		}
		// 将 extract 配置添加到 config 中
		if nodeConfig.Extract != nil {
			nodeConfigMap["extract"] = nodeConfig.Extract
		}

		node := dag.NewDGANodeWithConfig(
			nodeName,
			initialStatus,
			nodeConfig.Executor,
			nodeConfig.Image,
			nodeConfig.Steps,
			nodeConfigMap,
		)

		// 恢复运行时状态
		if nodeConfig.Runtime != nil {
			node.SetRuntimeStatus(nodeConfig.Runtime)
		}

		// 确保节点有ID
		node.EnsureIds()

		nodeMap[nodeName] = node
		graph.AddVertex(node)
	}

	// 解析图关系并添加边
	if config.Graph != "" {
		r.parseGraphEdges(graph, nodeMap, config.Graph)
	}

	return graph
}

// SetPipelineParam 设置 pipeline 的 param 值（内部使用）
func SetPipelineParam(pipeline dag.Pipeline, param map[string]interface{}) {
	if pipelineImpl, ok := pipeline.(*dag.PipelineImpl); ok {
		pipelineImpl.SetParam(param)
	}
}

// parseGraphEdges 解析图边关系
// 使用 mermaid-check 库解析 stateDiagram-v2 语法
// 支持从边标签中解析条件表达式，例如：A --> B: label[{eq .Param}]
func (r *RuntimeImpl) parseGraphEdges(graph dag.Graph, nodeMap map[string]dag.Node, graphStr string) {
	stateParser := parser.NewStateParser()
	diagram, err := stateParser.Parse(graphStr)
	if err != nil {
		// 解析失败时静默返回，不建立边关系
		return
	}

	// 转换为状态图
	stateDiagram, ok := diagram.(*ast.StateDiagram)
	if !ok {
		return
	}

	// 获取 dag.DGAGraph 用于记录入口/出口节点
	dgaGraph, isDGA := graph.(*dag.DGAGraph)

	// 遍历所有语句，提取转换关系
	for _, stmt := range stateDiagram.Statements {
		// 尝试转换为 Transition
		if transition, ok := stmt.(*ast.Transition); ok {
			// 记录入口节点（[*] --> X）
			if transition.From == "[*]" {
				if isDGA && transition.To != "[*]" {
					dgaGraph.AddEntryNode(transition.To)
				}
				continue
			}
			// 记录出口节点（X --> [*]）
			if transition.To == "[*]" {
				if isDGA {
					dgaGraph.AddExitNode(transition.From)
				}
				continue
			}

			srcNode, srcExists := nodeMap[transition.From]
			destNode, destExists := nodeMap[transition.To]

			if !srcExists || !destExists {
				continue
			}

			// 从 Label 中提取条件表达式
			expression := r.extractExpression(transition.Label)

			// 添加边关系（有条件表达式则创建条件边）
			var edge dag.Edge
			if expression != "" {
				edge = dag.NewConditionalEdge(srcNode, destNode, expression)
			} else {
				edge = dag.NewDGAEdge(srcNode, destNode)
			}
			_ = graph.AddEdge(edge)
		}
	}
}

// ExtractExpression 从边标签中提取条件表达式（公共函数供测试使用）
// 使用模板引擎的 Validate 方法验证表达式语法
// 先检查是否包含模板标记 {{ 或 {%，再使用模板引擎验证
func ExtractExpression(label string) string {
	if label == "" {
		return ""
	}

	// 检查是否包含模板表达式标记 {{ 或 {%
	if !strings.Contains(label, "{{") && !strings.Contains(label, "{%") {
		return ""
	}

	// 使用模板引擎验证表达式语法
	engine := template.NewPongo2TemplateEngine()
	if err := engine.Validate(label); err == nil {
		return label
	}
	return ""
}

// extractExpression 从边标签中提取条件表达式（内部使用）
// 使用模板引擎的Validate方法验证label是否是有效的模板表达式
func (r *RuntimeImpl) extractExpression(label string) string {
	if label == "" {
		return ""
	}

	// 检查是否包含模板表达式标记 {{ 或 {%
	if !strings.Contains(label, "{{") && !strings.Contains(label, "{%") {
		return ""
	}

	// 不能用 r.getTemplateEngine()：buildGraph 可能在持有 r.mu 写锁的
	// preparePipeline 中被调用，内部再 RLock 会自死锁；
	// ModifyGraph 路径持读锁时若有写者等待也会死锁。
	// Validate 是无状态语法校验，与包级 ExtractExpression 一致，直接新建引擎。
	engine := template.NewPongo2TemplateEngine()

	// 使用模板引擎验证label是否是有效的模板表达式
	if err := engine.Validate(label); err == nil {
		return label
	}
	return ""
}

// ModifyGraph 对暂停或停止的流水线执行图修改（原子操作）
func (r *RuntimeImpl) ModifyGraph(ctx context.Context, id string, modifications dag.GraphModifications) error {
	r.mu.RLock()
	pipeline, exists := r.pipelines[id]
	config, configExists := r.pipelineConfigs[id]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("pipeline with id %s not found", id)
	}

	// 校验可修改状态
	if !pipeline.IsModifiable() {
		return core.ErrPipelineRunning
	}

	graph := pipeline.GetGraph()

	// 快照当前状态用于回滚
	snapshotNodes := graph.Nodes()
	snapshotEdges := graph.Edges()

	// 回滚函数
	rollback := func() {
		// 恢复被删除的节点
		for _, node := range snapshotNodes {
			if _, ok := graph.GetNode(node.Id()); !ok {
				graph.AddVertex(node)
			}
		}
		// 恢复被删除的边
		for _, edge := range snapshotEdges {
			if _, ok := graph.GetEdge(edge.Source().Id(), edge.Target().Id()); !ok {
				_ = graph.AddEdge(edge)
			}
		}
	}

	// 1. 先删除边（在删除节点之前，避免悬空引用）
	for _, removal := range modifications.RemoveEdges {
		if err := graph.RemoveEdge(removal.Source, removal.Target); err != nil {
			rollback()
			return fmt.Errorf("failed to remove edge %s->%s: %w", removal.Source, removal.Target, err)
		}
	}

	// 2. 删除节点（自动删除关联边）
	for _, nodeID := range modifications.RemoveNodes {
		if err := graph.RemoveVertex(nodeID); err != nil {
			rollback()
			return fmt.Errorf("failed to remove node %s: %w", nodeID, err)
		}
	}

	// 3. 添加新节点
	nodeMap := make(map[string]dag.Node)
	for _, nodeConfig := range modifications.AddNodes {
		// 确保步骤有 ID
		for i := range nodeConfig.Steps {
			if nodeConfig.Steps[i].Id == "" {
				nodeConfig.Steps[i].Id = core.NewUUID()
			}
		}

		nodeConfigMap := make(map[string]any)
		for k, v := range nodeConfig.Config {
			nodeConfigMap[k] = v
		}
		if nodeConfig.Extract != nil {
			nodeConfigMap["extract"] = nodeConfig.Extract
		}

		// 使用 Name 作为节点 ID（与 buildGraph 一致）
		nodeName := nodeConfig.Name
		if nodeName == "" {
			nodeName = nodeConfig.Id
		}

		node := dag.NewDGANodeWithConfig(
			nodeName,
			core.StatusUnknown,
			nodeConfig.Executor,
			nodeConfig.Image,
			nodeConfig.Steps,
			nodeConfigMap,
		)
		node.EnsureIds()
		nodeMap[nodeName] = node
		graph.AddVertex(node)
	}

	// 4. 添加新边
	for _, edgeMod := range modifications.AddEdges {
		srcNode, ok := graph.GetNode(edgeMod.Source)
		if !ok {
			rollback()
			return fmt.Errorf("source node %s not found for edge", edgeMod.Source)
		}
		destNode, ok := graph.GetNode(edgeMod.Target)
		if !ok {
			rollback()
			return fmt.Errorf("target node %s not found for edge", edgeMod.Target)
		}

		var edge dag.Edge
		if edgeMod.Expression != "" {
			edge = dag.NewConditionalEdge(srcNode, destNode, edgeMod.Expression)
		} else {
			edge = dag.NewDGAEdge(srcNode, destNode)
		}

		if err := graph.AddEdge(edge); err != nil {
			rollback()
			return fmt.Errorf("failed to add edge %s->%s: %w", edgeMod.Source, edgeMod.Target, err)
		}
	}

	// 5. 解析 Mermaid 图片段
	if modifications.AddGraph != "" {
		// 合并已有的和新创建的 nodeMap
		existingNodes := graph.Nodes()
		for k, v := range existingNodes {
			nodeMap[k] = v
		}
		r.parseGraphEdges(graph, nodeMap, modifications.AddGraph)
	}

	// 6. 校验图结构：允许条件回边（循环图），拒绝无条件环
	if graph.HasCycle() {
		if dga, ok := graph.(*dag.DGAGraph); ok && dga.IsCyclic() {
			// 条件回边产生的环，允许
		} else {
			rollback()
			return core.ErrHasCycle
		}
	}

	// 7. 更新存储的配置（保证 ExportConfig 准确）
	if configExists {
		// 更新 Nodes 配置
		if config.Nodes == nil {
			config.Nodes = make(map[string]core.NodeConfig)
		}
		for _, nodeID := range modifications.RemoveNodes {
			delete(config.Nodes, nodeID)
		}
		for _, nodeConfig := range modifications.AddNodes {
			nodeName := nodeConfig.Name
			if nodeName == "" {
				nodeName = nodeConfig.Id
			}
			config.Nodes[nodeName] = nodeConfig
		}
	}

	// 8. 触发图修改事件
	if pipelineImpl, ok := pipeline.(*dag.PipelineImpl); ok {
		pipelineImpl.NotifyEvent(dag.PipelineGraphModified)
	}

	return nil
}

// UpdateConfig 通过新的 YAML 配置自动比对差异并更新流水线图
// 已执行的节点不允许删除或替换，只允许修改尚未运行的节点
// 除 Nodes 和 dag.Graph 外的其他配置字段不可更新
func (r *RuntimeImpl) UpdateConfig(ctx context.Context, id string, newConfigYAML string) error {
	r.mu.RLock()
	pipeline, exists := r.pipelines[id]
	oldConfig, configExists := r.pipelineConfigs[id]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("pipeline with id %s not found", id)
	}

	if !pipeline.IsModifiable() {
		return core.ErrPipelineRunning
	}

	newConfig, err := r.parseConfig(newConfigYAML)
	if err != nil {
		return fmt.Errorf("failed to parse new config: %w", err)
	}

	if configExists {
		if err := validateImmutableFields(oldConfig, newConfig); err != nil {
			return err
		}
	}

	graph := pipeline.GetGraph()
	mods, err := r.computeNodeModifications(oldConfig, newConfig, graph)
	if err != nil {
		return err
	}

	if newConfig.Graph != oldConfig.Graph {
		edges := graph.Edges()
		for _, edge := range edges {
			mods.RemoveEdges = append(mods.RemoveEdges, dag.EdgeRemoval{
				Source: edge.Source().Id(),
				Target: edge.Target().Id(),
			})
		}
		if newConfig.Graph != "" {
			mods.AddGraph = newConfig.Graph
		}
	}

	if mods.IsEmpty() {
		return nil
	}

	if err := r.ModifyGraph(ctx, id, mods); err != nil {
		return err
	}

	r.mu.Lock()
	if config, ok := r.pipelineConfigs[id]; ok {
		config.Graph = newConfig.Graph
	}
	r.mu.Unlock()

	return nil
}

// validateImmutableFields 校验不可变字段是否被修改
func validateImmutableFields(old, new *core.PipelineConfig) error {
	if old.Version != new.Version {
		return fmt.Errorf("%w: Version cannot be updated", core.ErrImmutableField)
	}
	if old.Name != new.Name {
		return fmt.Errorf("%w: Name cannot be updated", core.ErrImmutableField)
	}
	if old.MaxLoopIterations != new.MaxLoopIterations {
		return fmt.Errorf("%w: MaxLoopIterations cannot be updated", core.ErrImmutableField)
	}
	// nil 与空 map 视为相等：首次运行经 renderConfig 后 Param 被归一化为非 nil
	// 空 map，而新配置仅解析时为 nil，直接 DeepEqual 会误判
	if !reflect.DeepEqual(emptyMapToNil(old.Param), emptyMapToNil(new.Param)) {
		return fmt.Errorf("%w: Param cannot be updated", core.ErrImmutableField)
	}
	if !reflect.DeepEqual(normalizeExecutors(old.Executors), normalizeExecutors(new.Executors)) {
		return fmt.Errorf("%w: Executors cannot be updated", core.ErrImmutableField)
	}
	oldLog, newLog := old.Logging, new.Logging
	if len(oldLog.Headers) == 0 {
		oldLog.Headers = nil
	}
	if len(newLog.Headers) == 0 {
		newLog.Headers = nil
	}
	if !reflect.DeepEqual(oldLog, newLog) {
		return fmt.Errorf("%w: Logging cannot be updated", core.ErrImmutableField)
	}
	oldAI, newAI := old.AI, new.AI
	if len(oldAI.Constraints) == 0 {
		oldAI.Constraints = nil
	}
	if len(newAI.Constraints) == 0 {
		newAI.Constraints = nil
	}
	if !reflect.DeepEqual(oldAI, newAI) {
		return fmt.Errorf("%w: AI cannot be updated", core.ErrImmutableField)
	}
	oldMeta, newMeta := old.Metadate, new.Metadate
	oldMeta.Data = emptyMapToNil(oldMeta.Data)
	newMeta.Data = emptyMapToNil(newMeta.Data)
	if !reflect.DeepEqual(oldMeta, newMeta) {
		return fmt.Errorf("%w: Metadate cannot be updated", core.ErrImmutableField)
	}
	return nil
}

// emptyMapToNil 将空 map 归一化为 nil（用于不可变字段的稳定比较）
func emptyMapToNil(m map[string]interface{}) map[string]interface{} {
	if len(m) == 0 {
		return nil
	}
	return m
}

// normalizeExecutors 归一化 Executors 配置：空 map 为 nil、各 entry 的 Config 空 map 为 nil
func normalizeExecutors(m map[string]core.ExecutorConfig) map[string]core.ExecutorConfig {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]core.ExecutorConfig, len(m))
	for k, v := range m {
		v.Config = emptyMapToNil(v.Config)
		out[k] = v
	}
	return out
}

// computeNodeModifications 比对新旧节点配置，计算差异
func (r *RuntimeImpl) computeNodeModifications(oldConfig, newConfig *core.PipelineConfig, graph dag.Graph) (dag.GraphModifications, error) {
	var mods dag.GraphModifications

	oldNodes := oldConfig.Nodes
	if oldNodes == nil {
		oldNodes = make(map[string]core.NodeConfig)
	}
	newNodes := newConfig.Nodes
	if newNodes == nil {
		newNodes = make(map[string]core.NodeConfig)
	}

	// 找被删除的节点（old 有，new 没有）
	for nodeName := range oldNodes {
		if _, exists := newNodes[nodeName]; !exists {
			node, ok := graph.GetNode(nodeName)
			if ok && isNodeExecuted(node) {
				return mods, fmt.Errorf("%w: cannot remove node %q (status: %s)",
					core.ErrNodeAlreadyExecuted, nodeName, node.GetRuntimeStatus().Status)
			}
			mods.RemoveNodes = append(mods.RemoveNodes, nodeName)
		}
	}

	// 找新增/修改的节点
	for nodeName, newNodeConfig := range newNodes {
		// 确保 Name 字段与 map key 一致（与 buildGraph 行为一致）
		if newNodeConfig.Name == "" {
			newNodeConfig.Name = nodeName
		}
		oldNodeConfig, exists := oldNodes[nodeName]
		if !exists {
			mods.AddNodes = append(mods.AddNodes, newNodeConfig)
		} else {
			if !nodeConfigEqual(oldNodeConfig, newNodeConfig) {
				node, ok := graph.GetNode(nodeName)
				if ok && isNodeExecuted(node) {
					return mods, fmt.Errorf("%w: cannot modify node %q (status: %s)",
						core.ErrNodeAlreadyExecuted, nodeName, node.GetRuntimeStatus().Status)
				}
				mods.RemoveNodes = append(mods.RemoveNodes, nodeName)
				mods.AddNodes = append(mods.AddNodes, newNodeConfig)
			}
		}
	}

	return mods, nil
}

// nodeConfigEqual 比较两个 core.NodeConfig 是否相等（忽略 Runtime 字段）
func nodeConfigEqual(a, b core.NodeConfig) bool {
	aCopy := a
	bCopy := b
	// 忽略运行时状态
	aCopy.Runtime = nil
	bCopy.Runtime = nil
	// 忽略 Config（interface{} 类型的 map DeepEqual 不稳定）
	aCopy.Config = nil
	bCopy.Config = nil
	// 忽略 Description、Id 和 Name（不影响执行，Name 从 map key 派生）
	aCopy.Description = ""
	bCopy.Description = ""
	aCopy.Id = ""
	bCopy.Id = ""
	aCopy.Name = ""
	bCopy.Name = ""
	// 忽略步骤 ID（运行时分配，非用户配置；buildGraph/ModifyGraph 会回填）
	aCopy.Steps = stripStepIDs(aCopy.Steps)
	bCopy.Steps = stripStepIDs(bCopy.Steps)
	return reflect.DeepEqual(aCopy, bCopy)
}

// stripStepIDs 返回清空 Id 的步骤副本；空切片归一化为 nil 保证 DeepEqual 稳定
func stripStepIDs(steps []core.Step) []core.Step {
	if len(steps) == 0 {
		return nil
	}
	out := make([]core.Step, len(steps))
	copy(out, steps)
	for i := range out {
		out[i].Id = ""
	}
	return out
}

// isNodeExecuted 判断节点是否已经执行过
func isNodeExecuted(node dag.Node) bool {
	status := node.GetRuntimeStatus()
	if status == nil {
		return false
	}
	switch status.Status {
	case core.StatusSuccess, core.StatusFailed, core.StatusCancelled, core.StatusRunning:
		return true
	default:
		return false
	}
}