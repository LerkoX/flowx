package dag

import (
	"context"
	"fmt"
	"sync"

	"github.com/LerkoX/flowx/core"
	"github.com/thoas/go-funk"
)

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
// 循环体 = 从回边 target 沿正向边可达、且能沿正向边到达 source 的节点集合
// （循环出口下游节点如 source 之后的 tail 链不在集合内，不会被重置重跑）
func (dga *DGAGraph) LoopNodeSet(backEdge Edge) map[string]bool {
	dga.mu.RLock()
	defer dga.mu.RUnlock()
	return dga.loopNodeSetUnlocked(backEdge)
}

// loopNodeSetUnlocked 计算循环节点集合（无锁版本）
func (dga *DGAGraph) loopNodeSetUnlocked(backEdge Edge) map[string]bool {
	target := backEdge.Target().Id()
	source := backEdge.Source().Id()

	forwardGraph := dga.buildForwardGraph()

	// 从 target 出发沿正向边可达的节点
	reachableFromTarget := bfsReachable(forwardGraph, target)

	// 能沿正向边到达 source 的节点（在反向图上从 source BFS）
	reverseGraph := make(map[string][]string)
	for src, dests := range forwardGraph {
		for _, dest := range dests {
			reverseGraph[dest] = append(reverseGraph[dest], src)
		}
	}
	canReachSource := bfsReachable(reverseGraph, source)

	// 循环体 = 两集合交集；source 与 target 自身始终在集合中
	result := make(map[string]bool)
	for id := range reachableFromTarget {
		if canReachSource[id] {
			result[id] = true
		}
	}
	result[source] = true
	result[target] = true
	return result
}

// LoopExitNodeSet 计算循环出口下游节点集合：
// 从回边 source 沿正向边可达、但不属于任何循环体的节点。
// 这些节点应在循环条件不满足退出后再执行，而非循环迭代期间反复执行
func (dga *DGAGraph) LoopExitNodeSet(backEdges []Edge) map[string]bool {
	dga.mu.RLock()
	defer dga.mu.RUnlock()

	forwardGraph := dga.buildForwardGraph()
	result := make(map[string]bool)
	for _, backEdge := range backEdges {
		for id := range bfsReachable(forwardGraph, backEdge.Source().Id()) {
			result[id] = true
		}
	}
	// 剔除循环体节点（循环体节点需要每轮重跑，不属于出口下游）
	for _, backEdge := range backEdges {
		for id := range dga.loopNodeSetUnlocked(backEdge) {
			delete(result, id)
		}
	}
	return result
}

// bfsReachable 从 start 出发沿邻接表 BFS，返回所有可达节点（含 start 自身）
func bfsReachable(adj map[string][]string, start string) map[string]bool {
	result := map[string]bool{start: true}
	queue := []string{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adj[current] {
			if !result[next] {
				result[next] = true
				queue = append(queue, next)
			}
		}
	}
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