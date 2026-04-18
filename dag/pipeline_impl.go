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
	"github.com/LerkoX/flowx/template"
	"github.com/thoas/go-funk"
)


// 预检查PipelineImpl是否实现了Pipeline接口
var _ Pipeline = (*PipelineImpl)(nil)
var _ Graph = (*DGAGraph)(nil)

// 保存了流水线的图结构
type DGAGraph struct {
	mu       sync.RWMutex
	nodes    map[string]Node
	edges    map[string]Edge            // edgeID -> Edge
	graph    map[string][]string        // src -> [dest1, dest2, ...] (保持兼容性)
	edgeMap  map[string]map[string]Edge // src -> dest -> Edge (快速查找)
	sequence []string
	hasCycle bool

	// 循环图支持
	backEdges  map[string]bool  // 被接受的回边 edgeID 集合（条件边产生的环）
	entryNodes map[string]bool  // [*] 指向的入口节点
	exitNodes  map[string]bool  // 指向 [*] 的出口节点
}

func NewDGAGraph() *DGAGraph {
	return &DGAGraph{
		nodes:      map[string]Node{},
		edges:      map[string]Edge{},
		graph:      map[string][]string{},
		edgeMap:    map[string]map[string]Edge{},
		sequence:   []string{},
		backEdges:  map[string]bool{},
		entryNodes: map[string]bool{},
		exitNodes:  map[string]bool{},
	}
}

// Nodes 返回所有的节点map
func (dga *DGAGraph) Nodes() map[string]Node {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	return funk.Map(dga.nodes, func(k string, v Node) (string, Node) {
		return k, v
	}).(map[string]Node)
}

// Edges 返回所有的边
func (dga *DGAGraph) Edges() []Edge {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	edges := make([]Edge, 0, len(dga.edges))
	for _, edge := range dga.edges {
		edges = append(edges, edge)
	}
	return edges
}

// AddVertex 向图中添加顶点（节点）
// 检查是否存在循环；如果存在循环，则返回 core.ErrHasCycle
// 否则返回 nil
func (dga *DGAGraph) AddVertex(node Node) {
	dga.mu.Lock()
	defer dga.mu.Unlock()
	dga.nodes[node.Id()] = node
	dga.graph[node.Id()] = []string{}
}

// AddEdge 向图中添加边
func (dga *DGAGraph) AddEdge(edge Edge) error {
	dga.mu.Lock()
	defer dga.mu.Unlock()

	src := edge.Source()
	dest := edge.Target()

	if _, ok := dga.nodes[src.Id()]; !ok {
		return fmt.Errorf("source vertex %s not found", src.Id())
	}
	if _, ok := dga.nodes[dest.Id()]; !ok {
		return fmt.Errorf("dest vertex %s not found", dest.Id())
	}

	// 添加到edges映射
	dga.edges[edge.ID()] = edge

	// 添加到graph映射（保持兼容性）
	dga.graph[src.Id()] = append(dga.graph[src.Id()], dest.Id())

	// 添加到edgeMap（快速查找）
	if dga.edgeMap[src.Id()] == nil {
		dga.edgeMap[src.Id()] = make(map[string]Edge)
	}
	dga.edgeMap[src.Id()][dest.Id()] = edge

	dga.hasCycle = dga.cycleCheck()
	if dga.hasCycle {
		// 条件边产生的环：标记为回边，允许
		if edge.Expression() != "" {
			dga.backEdges[edge.ID()] = true
			return nil
		}
		// 无条件环：拒绝（死循环）
		return core.ErrHasCycle
	}
	return nil
}

// Traversal 对图执行广度优先遍历
// 为图中的每个节点执行提供的 TraversalFn 函数
// 支持有环图和无环图：使用 forwardGraph（排除回边）计算层级
// 同一层级内的节点并发执行，层级之间串行执行
func (dga *DGAGraph) Traversal(ctx context.Context, evalCtx EvaluationContext, fn TraversalFn) error {
	dga.mu.RLock()
	defer dga.mu.RUnlock()

	levels, err := dga.traversalStepsUnlocked(evalCtx)
	if err != nil {
		return err
	}

	for _, level := range levels {
		var wg sync.WaitGroup
		var errMu sync.Mutex
		var firstErr error

		for _, nodeID := range level {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				if err := fn(ctx, dga.nodes[id]); err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
				}
			}(nodeID)
		}
		wg.Wait()
		if firstErr != nil {
			return firstErr
		}
	}

	return nil
}


// cycleCheck 检查有向无环图（DAG）中是否存在循环
// 如果找到循环则返回 true，否则返回 false
func (dga *DGAGraph) cycleCheck() bool {
	indeg := dga.getIndegrees()
	q := make([]string, 0)
	for v, d := range indeg {
		if d == 0 {
			q = append(q, v)
		}
	}
	visited := 0
	for len(q) > 0 {
		v := q[0]
		q = q[1:]
		visited++
		for _, n := range dga.graph[v] {
			indeg[n]--
			if indeg[n] == 0 {
				q = append(q, n)
			}
		}
	}
	return visited != len(dga.nodes)
}

// getIndegrees 计算所有节点的入度
// 返回一个 map，key 是节点ID，value 是入度值
func (dga *DGAGraph) getIndegrees() map[string]int {
	indeg := make(map[string]int)
	for v := range dga.nodes {
		indeg[v] = 0
	}
	for _, adj := range dga.graph {
		for _, n := range adj {
			indeg[n]++
		}
	}
	return indeg
}

// HasCycle 检查图中是否存在循环
func (dga *DGAGraph) HasCycle() bool {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	return dga.hasCycle
}

// GetNode 根据节点ID查找节点
func (dga *DGAGraph) GetNode(nodeID string) (Node, bool) {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	node, ok := dga.nodes[nodeID]
	return node, ok
}

// GetEdge 根据源节点和目标节点ID查找边
func (dga *DGAGraph) GetEdge(srcID, destID string) (Edge, bool) {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	if srcMap, ok := dga.edgeMap[srcID]; ok {
		if edge, ok := srcMap[destID]; ok {
			return edge, true
		}
	}
	return nil, false
}

// IncomingEdges 返回指向指定节点的所有边
func (dga *DGAGraph) IncomingEdges(nodeID string) []Edge {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	edges := make([]Edge, 0)
	for src, destMap := range dga.edgeMap {
		if edge, ok := destMap[nodeID]; ok {
			_ = src
			edges = append(edges, edge)
		}
	}
	return edges
}

// OutgoingEdges 返回从指定节点出发的所有边
func (dga *DGAGraph) OutgoingEdges(nodeID string) []Edge {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	edges := make([]Edge, 0)
	if destMap, ok := dga.edgeMap[nodeID]; ok {
		for _, edge := range destMap {
			edges = append(edges, edge)
		}
	}
	return edges
}

// RemoveVertex 删除节点及其所有关联边
func (dga *DGAGraph) RemoveVertex(nodeID string) error {
	dga.mu.Lock()
	defer dga.mu.Unlock()

	// 校验节点存在
	if _, ok := dga.nodes[nodeID]; !ok {
		return core.ErrNodeNotFound
	}

	// 删除以该节点为目标的边（从其他节点的 edgeMap 和 graph 中清理）
	for src, destMap := range dga.edgeMap {
		if edge, ok := destMap[nodeID]; ok {
			delete(dga.edges, edge.ID())
			delete(destMap, nodeID)
			// 从 graph[src] 中删除 nodeID
			dga.graph[src] = removeFromSlice(dga.graph[src], nodeID)
		}
	}

	// 删除以该节点为源的边
	if destMap, ok := dga.edgeMap[nodeID]; ok {
		for _, edge := range destMap {
			delete(dga.edges, edge.ID())
		}
		delete(dga.edgeMap, nodeID)
	}

	// 删除 graph 中的出边列表
	delete(dga.graph, nodeID)

	// 删除节点
	delete(dga.nodes, nodeID)

	// 从 sequence 中删除
	dga.sequence = removeFromSlice(dga.sequence, nodeID)

	// 重新计算环检测
	dga.hasCycle = dga.cycleCheck()

	return nil
}

// RemoveEdge 删除指定的边
func (dga *DGAGraph) RemoveEdge(srcID, destID string) error {
	dga.mu.Lock()
	defer dga.mu.Unlock()

	// 查找边
	srcMap, ok := dga.edgeMap[srcID]
	if !ok {
		return core.ErrEdgeNotFound
	}
	edge, ok := srcMap[destID]
	if !ok {
		return core.ErrEdgeNotFound
	}

	// 删除
	delete(dga.edges, edge.ID())
	delete(srcMap, destID)
	if len(srcMap) == 0 {
		delete(dga.edgeMap, srcID)
	}

	// 从 graph 中删除
	dga.graph[srcID] = removeFromSlice(dga.graph[srcID], destID)

	// 重新计算环检测
	dga.hasCycle = dga.cycleCheck()

	return nil
}

// IsCyclic 返回图是否有被接受的回边（条件循环节点）
func (dga *DGAGraph) IsCyclic() bool {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	return len(dga.backEdges) > 0
}

// BackEdges 返回所有被接受的回边
func (dga *DGAGraph) BackEdges() []Edge {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	edges := make([]Edge, 0, len(dga.backEdges))
	for edgeID := range dga.backEdges {
		if edge, ok := dga.edges[edgeID]; ok {
			edges = append(edges, edge)
		}
	}
	return edges
}

// addEntryNode 添加入口节点（从 [*] 指向的节点）
func (dga *DGAGraph) AddEntryNode(nodeID string) {
	dga.mu.Lock()
	defer dga.mu.Unlock()
	dga.entryNodes[nodeID] = true
}

// addExitNode 添加出口节点（指向 [*] 的节点）
func (dga *DGAGraph) AddExitNode(nodeID string) {
	dga.mu.Lock()
	defer dga.mu.Unlock()
	dga.exitNodes[nodeID] = true
}

// EntryNodes 返回入口节点列表
func (dga *DGAGraph) EntryNodes() []string {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	nodes := make([]string, 0, len(dga.entryNodes))
	for id := range dga.entryNodes {
		nodes = append(nodes, id)
	}
	return nodes
}

// ExitNodes 返回出口节点列表
func (dga *DGAGraph) ExitNodes() []string {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	nodes := make([]string, 0, len(dga.exitNodes))
	for id := range dga.exitNodes {
		nodes = append(nodes, id)
	}
	return nodes
}

// buildForwardGraph 构建排除回边的邻接表
func (dga *DGAGraph) buildForwardGraph() map[string][]string {
	forwardGraph := make(map[string][]string)
	for src, dests := range dga.graph {
		for _, dest := range dests {
			// 检查这条边是否是回边
			edgeID := src + "->" + dest
			if !dga.backEdges[edgeID] {
				forwardGraph[src] = append(forwardGraph[src], dest)
			}
		}
		// 确保没有出边的节点也在 forwardGraph 中
		if _, ok := forwardGraph[src]; !ok {
			forwardGraph[src] = []string{}
		}
	}
	return forwardGraph
}

// LoopNodeSet 计算回边涉及的循环节点集合
// 从回边的 target 出发，沿 forward edges BFS 到达 source 为止的所有节点
func (dga *DGAGraph) LoopNodeSet(backEdge Edge) map[string]bool {
	dga.mu.RLock()
	defer dga.mu.RUnlock()

	target := backEdge.Target().Id()
	source := backEdge.Source().Id()

	result := make(map[string]bool)
	forwardGraph := dga.buildForwardGraph()

	// BFS 从 target 出发，收集可达的所有节点
	queue := []string{target}
	visited := map[string]bool{target: true}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result[current] = true

		for _, next := range forwardGraph[current] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}

	// source 也应该在集合中
	result[source] = true

	return result
}

// removeFromSlice 从字符串切片中删除指定元素
func removeFromSlice(slice []string, item string) []string {
	for i, v := range slice {
		if v == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// TraversalSteps 计算 BFS 层级执行计划
// 返回 [][]string，每个子切片是一层可并发执行的节点ID列表
// 支持循环图：使用 forwardGraph（排除回边）计算层级，使用 entryNodes 作为起始节点
// traversalStepsUnlocked 计算 BFS 层级执行计划（无锁版本）
// 供 Traversal 和 TraversalSteps 共用
func (dga *DGAGraph) traversalStepsUnlocked(evalCtx EvaluationContext) ([][]string, error) {
	if len(dga.nodes) == 0 {
		return nil, nil
	}

	// 使用 forwardGraph（排除回边）计算入度
	forwardGraph := dga.buildForwardGraph()
	indeg := make(map[string]int)
	for v := range dga.nodes {
		indeg[v] = 0
	}
	for _, adj := range forwardGraph {
		for _, n := range adj {
			indeg[n]++
		}
	}

	// 收集起始节点：优先使用 entryNodes（[*] 指向的节点），否则使用入度为0的节点
	startNodes := make([]string, 0)
	if len(dga.entryNodes) > 0 {
		for id := range dga.entryNodes {
			if _, ok := dga.nodes[id]; ok {
				startNodes = append(startNodes, id)
			}
		}
	}
	if len(startNodes) == 0 {
		for v, d := range indeg {
			if d == 0 {
				startNodes = append(startNodes, v)
			}
		}
	}

	if len(startNodes) == 0 {
		return nil, nil
	}

	var levels [][]string
	visited := make(map[string]bool)
	queue := make([]string, 0)

	// 第一层：起始节点
	for _, id := range startNodes {
		visited[id] = true
		queue = append(queue, id)
	}
	if len(queue) > 0 {
		levels = append(levels, append([]string{}, queue...))
	}

	// BFS 遍历（使用 forwardGraph 代替 dga.graph）
	for len(queue) > 0 {
		var nextLevel []string
		for _, vertexFocus := range queue {
			for _, neighbor := range forwardGraph[vertexFocus] {
				if visited[neighbor] {
					continue
				}

				// 评估条件边（跳过回边）
				if edge, ok := dga.edgeMap[vertexFocus][neighbor]; ok && edge.Expression() != "" {
					// 跳过回边的条件评估（回边由循环执行引擎处理）
					if dga.backEdges[edge.ID()] {
						continue
					}
					result, err := edge.Evaluate(evalCtx)
					if err != nil {
						return nil, fmt.Errorf("failed to evaluate edge condition %s->%s: %w",
							vertexFocus, neighbor, err)
					}
					if !result {
						continue
					}
				}

				indeg[neighbor]--
				if indeg[neighbor] == 0 {
					visited[neighbor] = true
					nextLevel = append(nextLevel, neighbor)
				}
			}
		}

		if len(nextLevel) > 0 {
			levels = append(levels, nextLevel)
		}
		queue = nextLevel
	}

	return levels, nil
}

// TraversalSteps 计算 BFS 层级执行计划
// 返回 [][]string，每个子切片是一层可并发执行的节点ID列表
func (dga *DGAGraph) TraversalSteps(evalCtx EvaluationContext) [][]string {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	levels, _ := dga.traversalStepsUnlocked(evalCtx)
	return levels
}


type PipelineImpl struct {
	id               string
	graph            Graph
	status           string
	metadata         Metadata
	metadataStore    metadata.MetadataStore
	listening        ListeningFn
	listener         Listener
	doneChan         chan struct{}
	cancelFunc       context.CancelFunc
	mu               sync.RWMutex
	executorProvider ExecutorProvider
	executors        map[string]executor.Executor    // 缓存已创建的executor
	param            map[string]core.FieldItem // 存储渲染后的Param值
	templateEngine   template.TemplateEngine         // 模板引擎
	cleanupOnce      sync.Once              // 保护清理操作只执行一次
	pauseMu          sync.Mutex             // 保护暂停/恢复操作的序列化
	pauseChan        chan struct{}           // 暂停信号通道
	resumeChan       chan struct{}           // 恢复信号通道
	currentLevel     int                    // 记录当前执行到的BFS层级（用于暂停恢复）
	maxLoopIter      int                    // 循环图最大迭代次数
	pusher           logger.Pusher           // 日志推送器
}

func NewPipeline(ctx context.Context) Pipeline {
	return &PipelineImpl{
		id:           core.NewUUID(),
		executors:    make(map[string]executor.Executor),
		doneChan:     make(chan struct{}),
		pauseChan:    make(chan struct{}),
		resumeChan:   make(chan struct{}),
		maxLoopIter:  100, // 默认最大迭代次数
	}
}

// SetParam 设置 param 值
func (p *PipelineImpl) SetParam(param map[string]interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 将 map[string]interface{} 转换为 map[string]FieldItem
	p.param = make(map[string]core.FieldItem)
	for k, v := range param {
		p.param[k] = core.ConvertToFieldItem(v)
	}
}

// SetMaxLoopIterations 设置循环图最大迭代次数
func (p *PipelineImpl) SetMaxLoopIterations(max int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if max > 0 {
		p.maxLoopIter = max
	}
}

// Id 返回流水线的ID
func (p *PipelineImpl) Id() string {
	return p.id
}

// GetGraph 返回流水线的图结构
func (p *PipelineImpl) GetGraph() Graph {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.graph
}

// SetGraph 设置流水线的图结构
func (p *PipelineImpl) SetGraph(graph Graph) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.graph = graph
}

// Status 返回流水线的整体状态
func (p *PipelineImpl) Status() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

// SetMetadata 设置流水线的元数据存储
func (p *PipelineImpl) SetMetadata(store metadata.MetadataStore) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metadataStore = store
}

// Metadata 获取流水线的元数据
func (p *PipelineImpl) Metadata() Metadata {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.metadata == nil {
		p.metadata = make(Metadata)
	}

	// 如果有 metadataStore，从 store 加载数据
	if p.metadataStore != nil {
		// 从 metadata.InConfigMetadataStore 加载所有数据
		if inConfigStore, ok := p.metadataStore.(*metadata.InConfigMetadataStore); ok {
			for k, v := range inConfigStore.GetAll() {
				// 将 string 值转换为 FieldItem
				p.metadata[k] = core.FieldItem{
					Value:       v,
					Description: "",
					SrcNode:     "",
				}
			}
		}
	}

	// 返回元数据的拷贝，避免并发修改问题
	result := make(Metadata)
	for k, v := range p.metadata {
		result[k] = v
	}
	return result
}

// Listening 设置流水线执行事件监听器
func (p *PipelineImpl) Listening(fn Listener) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.listener = fn
}

// Done 返回一个通道，用于通知流水线何时完成
func (p *PipelineImpl) Done() <-chan struct{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.doneChan
}

// shouldSkipNode 检查节点是否应该跳过执行
func (p *PipelineImpl) shouldSkipNode(node Node) bool {
	runtimeStatus := node.GetRuntimeStatus()
	if runtimeStatus == nil {
		return false
	}
	// SUCCESS、FAILED、CANCELLED 状态的节点都跳过
	// RUNNING 状态的节点需要恢复（不跳过）
	switch runtimeStatus.Status {
	case core.StatusSuccess, core.StatusFailed, core.StatusCancelled:
		return true
	default:
		return false
	}
}

// Run 执行流水线
func (p *PipelineImpl) Run(ctx context.Context) error {
	p.mu.Lock()
	ctx, cancel := context.WithCancel(ctx)
	p.cancelFunc = cancel
	p.status = core.StatusRunning
	p.mu.Unlock()

	defer func() {
		p.cleanupOnce.Do(func() {
			close(p.doneChan)

			p.mu.Lock()
			if p.cancelFunc != nil {
				p.cancelFunc()
				p.cancelFunc = nil
			}
			p.mu.Unlock()

			// 清理所有executor
			p.cleanupExecutors(ctx)
		})
	}()

	// 通知流水线开始
	p.NotifyEvent(PipelineStart)

	// 创建求值上下文
	evalCtx := NewEvaluationContext().WithPipeline(p)

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
		p.NotifyEvent(PipelineFinish)
		return err
	}

		// 逐层 BFS 执行，同时支持有环图和无环图
		startLevel := p.restoreTraversalState()
		if err := p.runLevelByLevel(ctx, dgaGraph, evalCtx, startLevel); err != nil {
			p.mu.Lock()
			p.status = core.StatusFailed
			p.mu.Unlock()
			p.NotifyEvent(PipelineFinish)
			return err
		}

	// 通知流水线完成
	p.mu.Lock()
	p.status = core.StatusSuccess
	p.mu.Unlock()
	p.NotifyEvent(PipelineFinish)
	return nil
}

// runLevelByLevel 逐层执行 BFS 遍历，支持暂停/恢复和循环图
// 无环图：执行完所有层级后直接返回
// 有环图：执行完所有层级后评估回边条件，满足则重置循环节点并继续迭代
func (p *PipelineImpl) runLevelByLevel(ctx context.Context, dgaGraph *DGAGraph, evalCtx EvaluationContext, startLevel int) error {
	iteration := 0
	p.mu.RLock()
	maxIter := p.maxLoopIter
	p.mu.RUnlock()

	for {
		evalCtx = evalCtx.WithIteration(iteration)

		// 计算初始层级
		levels := dgaGraph.TraversalSteps(evalCtx)
		if len(levels) == 0 {
			return nil
		}

		var firstErr error
		levelIdx := startLevel

		for levelIdx < len(levels) {
			// 检查暂停信号
			select {
			case <-p.pauseChan:
				// 保存当前层级并进入暂停状态
				p.mu.Lock()
				p.currentLevel = levelIdx
				p.status = core.StatusPaused
				p.mu.Unlock()
				p.NotifyEvent(PipelinePaused)

				// 等待恢复信号
				<-p.resumeChan

				// 恢复运行
				p.mu.Lock()
				p.status = core.StatusRunning
				p.pauseChan = make(chan struct{})
				p.resumeChan = make(chan struct{})
				p.mu.Unlock()
				p.NotifyEvent(PipelineResumed)

				// 重新计算层级（图可能已被修改）
				levels = dgaGraph.TraversalSteps(evalCtx)
				if levelIdx >= len(levels) {
					return nil
				}
			default:
			}

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
				levels = dgaGraph.TraversalSteps(evalCtx)
			}
		}

		// 循环检测：评估回边条件
		backEdges := dgaGraph.BackEdges()
		if len(backEdges) == 0 {
			return nil // 无环图：直接返回
		}

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
			return nil // 条件不满足：循环结束
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

func (p *PipelineImpl) resetLoopNodes(dgaGraph *DGAGraph, loopNodes map[string]bool) {
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
func (p *PipelineImpl) makeTraversalFn(ctx context.Context) TraversalFn {
	return func(ctx context.Context, node Node) error {
		return p.executeNodeWithLifecycle(ctx, node)
	}
}

// executeNodeWithLifecycle 执行节点的完整生命周期（跳过检查→通知→执行→通知）
func (p *PipelineImpl) executeNodeWithLifecycle(ctx context.Context, node Node) error {
	// 检查 context 是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 检查是否应该跳过此节点
	if p.shouldSkipNode(node) {
		fmt.Printf("Skipping node %s (status: %s)\n", node.Id(), node.GetRuntimeStatus().Status)
		p.NotifyEvent(PipelineNodeFinish)
		return nil
	}

	// 通知节点开始
	p.NotifyEvent(PipelineNodeStart)
	fmt.Printf("Executing node: %s\n", node.Id())

	// 获取节点的executor配置
	executorName := node.GetExecutor()
	if executorName == "" {
		fmt.Printf("Node %s has no executor configured, skipping\n", node.Id())
		p.NotifyEvent(PipelineNodeFinish)
		return nil
	}

	// 获取或创建executor
	exec, err := p.getOrCreateExecutor(ctx, executorName)
	if err != nil {
		fmt.Printf("Failed to get executor for node %s: %v\n", node.Id(), err)
		return fmt.Errorf("failed to get executor for node %s: %w", node.Id(), err)
	}

	// 执行节点
	if err := p.executeNode(ctx, node, exec); err != nil {
		fmt.Printf("Node %s execution failed: %v\n", node.Id(), err)
		return err
	}

	// 通知节点完成
	p.NotifyEvent(PipelineNodeFinish)
	return nil
}

// Pause 暂停流水线，等待当前层执行完成后暂停
func (p *PipelineImpl) Pause() error {
	p.pauseMu.Lock()
	defer p.pauseMu.Unlock()

	p.mu.RLock()
	status := p.status
	p.mu.RUnlock()

	if status != core.StatusRunning {
		return fmt.Errorf("%w: current status is %s, expected RUNNING", core.ErrInvalidState, status)
	}

	// 发送暂停信号
	close(p.pauseChan)
	return nil
}

// Resume 恢复暂停的流水线
func (p *PipelineImpl) Resume(ctx context.Context) error {
	p.pauseMu.Lock()
	defer p.pauseMu.Unlock()

	p.mu.RLock()
	status := p.status
	p.mu.RUnlock()

	if status != core.StatusPaused && status != core.StatusStopped {
		return fmt.Errorf("%w: current status is %s, expected PAUSED or STOPPED", core.ErrInvalidState, status)
	}

	// 发送恢复信号
	close(p.resumeChan)
	return nil
}

// IsModifiable 判断当前是否可修改图
func (p *PipelineImpl) IsModifiable() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	switch p.status {
	case core.StatusPaused, core.StatusStopped, core.StatusFailed, core.StatusCancelled, core.StatusSuccess:
		return true
	default:
		return false
	}
}

// restoreTraversalState 恢复遍历状态并重置
func (p *PipelineImpl) restoreTraversalState() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	level := p.currentLevel
	p.currentLevel = 0
	return level
}

// executeNode 执行单个节点
func (p *PipelineImpl) executeNode(ctx context.Context, node Node, exec executor.Executor) error {
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

	// 3. 发送所有步骤命令
	go p.sendCommands(ctx, node, commandChan, steps)

	// 4. 等待并处理所有结果
	lastErr, fullOutput := p.waitForResults(ctx, node, exec, resultChan, steps)

	// 5. 从完整输出中提取元数据
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

	// 6. 更新节点最终状态
	if runtimeStatus = node.GetRuntimeStatus(); runtimeStatus != nil {
		p.updateNodeFinalStatus(runtimeStatus, lastErr)
		node.SetRuntimeStatus(runtimeStatus)
	}

	// 7. 关闭 inputChan
	close(inputChan)

	return lastErr
}

// initializeNodeRuntimeStatus 初始化节点运行时状态
func (p *PipelineImpl) initializeNodeRuntimeStatus(node Node, exec executor.Executor) *core.NodeRuntimeStatus {
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
func (p *PipelineImpl) setupExecutorChannels(ctx context.Context, exec executor.Executor, steps []core.Step) (chan any, chan any, chan []byte) {
	commandChan := make(chan any, len(steps))
	resultChan := make(chan any, len(steps)*10)
	inputChan := make(chan []byte, 100) // 输入通道，缓冲100条消息

	// 启动 executor 的 Transfer goroutine
	go exec.Transfer(ctx, resultChan, commandChan, inputChan)

	return commandChan, resultChan, inputChan
}

// sendCommands 发送所有步骤命令
func (p *PipelineImpl) sendCommands(ctx context.Context, node Node, commandChan chan any, steps []core.Step) {
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

		// 运行时渲染 step.Run，可以引用前面节点 extract 的数据
		renderedRun, err := p.renderStringWithRuntimeContext(step.Run)
		if err != nil {
			fmt.Printf("Failed to render step %s: %v, using original command\n", step.Name, err)
			renderedRun = step.Run // 渲染失败时使用原始命令
		}

		commandChan <- executor.CommandWrapper{
			StepName: step.Name,
			Command:  renderedRun,
		}
	}
}

// shouldSkipStep 检查步骤是否应该跳过执行
func (p *PipelineImpl) shouldSkipStep(node Node, stepName string) bool {
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
func (p *PipelineImpl) waitForResults(ctx context.Context, node Node, exec executor.Executor, resultChan chan any, steps []core.Step) (error, string) {
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
		return nil, ""
			}
	var allOutput strings.Builder

	for resultCount < expectedResults {
		select {
		case <-ctx.Done():
			p.handleCancellation(nodeContext{ctx: ctx, node: node})
			return ctx.Err(), allOutput.String()
		case result, ok := <-resultChan:
			if !ok {
				return lastErr, allOutput.String()
			}

			errCount := p.handleResult(ctx, node, exec, result, resultCount, steps)
			if errCount.err != nil {
				lastErr = errCount.err
			}
			resultCount = errCount.count
			allOutput.WriteString(errCount.output)
		}
	}

	return lastErr, allOutput.String()
}

// handleCancellation 处理节点取消
func (p *PipelineImpl) handleCancellation(ctx nodeContext) {
	runtimeStatus := ctx.node.GetRuntimeStatus()
	if runtimeStatus != nil {
		runtimeStatus.Status = core.StatusCancelled
		runtimeStatus.EndTime = time.Now().Format(time.RFC3339)
		ctx.node.SetRuntimeStatus(runtimeStatus)
	}
}

// handleResult 处理单个结果
func (p *PipelineImpl) handleResult(ctx context.Context, node Node, _ executor.Executor, result any, resultCount int, steps []core.Step) resultHandler {
	handler := resultHandler{count: resultCount}

	switch v := result.(type) {
	case error:
		handler.err = v
		handler.count++
	case *executor.StepResult:
		if v.Error != nil {
			handler.err = v.Error
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
		if p.pusher != nil {
			p.pusher.Push(ctx, logger.Entry{
				Level:   logger.LevelInfo,
				Message: output,
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
func (p *PipelineImpl) handleInputRequest(node Node, event *executor.InputRequestEvent) {
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
	p.NotifyEvent(PipelinePaused)
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
func (p *PipelineImpl) updateStepRuntimeStatus(node Node, step core.Step, result *executor.StepResult) {
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
func (p *PipelineImpl) updateNodeFinalStatus(runtimeStatus *core.NodeRuntimeStatus, err error) {
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

// 这个主要是在运行过程中节点状态或者流水线状态变化，就会触发这个函数
// 节点
// 我们就可以在这里做一些处理
// 执行ListeningFn函数
func (p *PipelineImpl) Notify() {
	p.mu.RLock()
	listening := p.listening
	listener := p.listener
	p.mu.RUnlock()

	// 如果设置了ListeningFn则调用它
	if listening != nil {
		listening(p)
	}

	// 如果设置了事件监听器则处理它
	if listener != nil {
		// 通知当前状态
		p.notifyCurrentStatus(listener)
	}
}

// 终止流水线
func (p *PipelineImpl) Cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancelFunc != nil {
		p.cancelFunc()
		p.status = core.StatusCancelled

		// 通知监听器关于取消事件
		if p.listener != nil {
			p.listener.Handle(p, core.EventPipelineCancelled)
		}

		if p.listening != nil {
			p.listening(p)
		}
	}
}

// notifyEvent 通知监听器特定事件
func (p *PipelineImpl) NotifyEvent(event Event) {
	p.mu.RLock()
	listener := p.listener
	listening := p.listening
	p.mu.RUnlock()

	if listener != nil {
		listener.Handle(p, event)
	}

	if listening != nil {
		listening(p)
	}
}

// notifyCurrentStatus 通知监听器当前流水线状态
func (p *PipelineImpl) notifyCurrentStatus(listener Listener) {
	// 此方法可用于通知详细的状态变化
	// 目前，它仅用当前流水线调用监听器
	listener.Handle(p, core.EventPipelineStatusUpdate)
}

// SetExecutorProvider 设置Executor提供者
func (p *PipelineImpl) SetExecutorProvider(provider ExecutorProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.executorProvider = provider
	p.executors = make(map[string]executor.Executor)
}

// getOrCreateExecutor 获取或创建Executor
func (p *PipelineImpl) getOrCreateExecutor(ctx context.Context, name string) (executor.Executor, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查缓存
	if executor, ok := p.executors[name]; ok {
		return executor, nil
	}

	// 使用provider创建executor
	if p.executorProvider == nil {
		return nil, fmt.Errorf("executor provider not set")
	}

	executor, err := p.executorProvider.GetExecutor(ctx, name)
	if err != nil {
		return nil, err
	}

	// 缓存executor
	p.executors[name] = executor
	return executor, nil
}

// cleanupExecutors 清理所有executor
func (p *PipelineImpl) cleanupExecutors(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for name, executor := range p.executors {
		if err := executor.Destruction(ctx); err != nil {
			fmt.Printf("failed to destroy executor %s: %v\n", name, err)
		}
	}
	p.executors = make(map[string]executor.Executor)
}

// SetTemplateEngine 设置模板引擎
func (p *PipelineImpl) SetTemplateEngine(engine template.TemplateEngine) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.templateEngine = engine
}

// GetTemplateEngine 获取模板引擎
func (p *PipelineImpl) GetTemplateEngine() template.TemplateEngine {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.templateEngine
}

// SetPusher 设置日志推送器
func (p *PipelineImpl) SetPusher(pusher logger.Pusher) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pusher = pusher
}

// buildRenderContext 构建渲染上下文，包含 Param 和动态 Metadata
func (p *PipelineImpl) buildRenderContext() map[string]any {
	ctx := make(map[string]any)

	// 添加 Param（提取 FieldItem.Value 用于模板渲染）
	p.mu.RLock()
	if p.param != nil {
		paramValues := make(map[string]any)
		for k, v := range p.param {
			paramValues[k] = core.GetValue(v.Value)
		}
		ctx["Param"] = paramValues
		// 同时展开 param 到顶层，支持直接引用
		for k, v := range paramValues {
			ctx[k] = v
		}
	}
	p.mu.RUnlock()

	// 添加 Metadata 和其他动态数据（这里 Metadata 返回的是拷贝，安全）
	metadata := p.Metadata()
	if metadata != nil {
		metadataValues := make(map[string]any)
		for k, v := range metadata {
			metadataValues[k] = core.GetValue(v.Value)
		}
		ctx["Metadata"] = metadataValues
		// 将平铺的 metadata 转换为嵌套结构
		// 例如：Node1.value = "42", Node1.message = "hello"
		// 转换为：Node1 = {"value": "42", "message": "hello"}
		for k, v := range metadataValues {
			// 检查键名是否包含点（节点ID.键名）
			if dotIdx := strings.LastIndex(k, "."); dotIdx > 0 {
				nodeID := k[:dotIdx]
				keyName := k[dotIdx+1:]
				// 如果节点ID 的嵌套对象不存在，创建它
				if _, exists := ctx[nodeID]; !exists {
					ctx[nodeID] = make(map[string]any)
				}
				// 将键值添加到节点的嵌套对象中
				if nodeObj, ok := ctx[nodeID].(map[string]any); ok {
					nodeObj[keyName] = v
				}
			} else {
				// 不包含点的键，直接添加
				ctx[k] = v
			}
		}
	}

	return ctx
}

// renderStringWithRuntimeContext 使用运行时上下文渲染字符串
func (p *PipelineImpl) renderStringWithRuntimeContext(template string) (string, error) {
	engine := p.GetTemplateEngine()
	if engine == nil {
		return template, nil // 没有模板引擎，返回原始值
	}
	ctx := p.buildRenderContext()
	return engine.EvaluateString(template, ctx)
}

// extractOutput 从节点输出中提取数据并保存到metadata
func (p *PipelineImpl) extractOutput(ctx context.Context, node Node, stepResult *executor.StepResult, fullOutput string) error {
	// 获取节点配置
	nodeConfig := node.GetConfig()
	if nodeConfig == nil {
		return nil
	}

	// 检查是否有提取配置
	extractConfig, hasExtract := nodeConfig["extract"]
	if !hasExtract || extractConfig == nil {
		return nil
	}

	// 创建提取器
	extractor, err := p.createExtractor(extractConfig)
	if err != nil {
		return fmt.Errorf("failed to create extractor: %w", err)
	}

	// 执行提取

	extracted, err := extractor.Extract(fullOutput)
	if err != nil {
		return fmt.Errorf("failed to extract data from node %s: %w", node.Id(), err)
	}

	// 保存到 metadata（加锁防止并发写入）
	if len(extracted) > 0 {
		p.mu.Lock()
		defer p.mu.Unlock()

		// 确保 metadata 已初始化
		if p.metadata == nil {
			p.metadata = make(Metadata)
		}

		// 保存到内存 metadata（使用 FieldItem，设置 SrcNode）
		for key, fieldItem := range extracted {
			metadataKey := fmt.Sprintf("%s.%s", node.Id(), key)
			// 设置 SrcNode 为当前节点 ID
			fieldItem.SrcNode = node.Id()
			// 将 Value 转换为字符串存储
			valueStr := fmt.Sprintf("%v", core.GetValue(fieldItem.Value))
			fieldItem.Value = valueStr
			p.metadata[metadataKey] = fieldItem
		}

		// 如果有 metadata store，同步保存
		if p.metadataStore != nil {
			for key, fieldItem := range extracted {
				metadataKey := fmt.Sprintf("%s.%s", node.Id(), key)
				valueStr := fmt.Sprintf("%v", core.GetValue(fieldItem.Value))
				if err := p.metadataStore.Set(ctx, metadataKey, valueStr); err != nil {
					fmt.Printf("Warning: Failed to save extracted data to store: %v\n", err)
				}
			}
		}
	}

	return nil
}

// createExtractor 根据配置创建提取器
func (p *PipelineImpl) createExtractor(extractConfig interface{}) (OutputExtractor, error) {
	if extractConfig == nil {
		return nil, nil
	}

	var configMap map[string]interface{}

	// 尝试解析为 map[string]interface{}
	if m, ok := extractConfig.(map[string]interface{}); ok {
		configMap = m
	} else if extractPtr, ok := extractConfig.(*core.ExtractConfig); ok && extractPtr != nil {
		// 转换 *core.ExtractConfig 到 map[string]interface{}
		configMap = make(map[string]interface{})
		if extractPtr.Type != "" {
			configMap["type"] = extractPtr.Type
		}
		if extractPtr.Patterns != nil {
			// 将 Patterns 转换为 map[string]interface{}
			patternsMap := make(map[string]interface{})
			for k, v := range extractPtr.Patterns {
				patternsMap[k] = v
			}
			configMap["patterns"] = patternsMap
		}
		if extractPtr.MaxOutputSize > 0 {
			configMap["maxOutputSize"] = extractPtr.MaxOutputSize
		}
	} else {
		return nil, fmt.Errorf("invalid extract config format")
	}

	// 获取提取类型
	extractType := "codec-block" // 默认类型
	if typeVal, ok := configMap["type"]; ok {
		if typeStr, ok := typeVal.(string); ok {
			extractType = typeStr
		}
	}

	// 获取输出大小限制
	maxOutputSize := 1024 * 1024 // 默认 1MB
	if sizeVal, ok := configMap["maxOutputSize"]; ok {
		if size, ok := sizeVal.(int); ok {
			maxOutputSize = size
		}
	}

	switch extractType {
	case "codec-block":
		return NewCodecBlockExtractor(maxOutputSize), nil

	case "regex":
		patterns := make(map[string]string)
		if patternsVal, ok := configMap["patterns"]; ok {
			if patternsMap, ok := patternsVal.(map[string]interface{}); ok {
				for k, v := range patternsMap {
					if patternStr, ok := v.(string); ok {
						patterns[k] = patternStr
					}
				}
			}
		}
		if len(patterns) == 0 {
			return nil, fmt.Errorf("regex extractor requires patterns")
		}
		return NewRegexExtractor(patterns, maxOutputSize)

	default:
		return nil, fmt.Errorf("unsupported extract type: %s", extractType)
	}
}
