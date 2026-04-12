package dag

import (
	"context"
	"sync"
	"testing"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/executor/provider"
)

// TestDGAGraph_ConditionalBackEdge 测试条件回边被接受
func TestDGAGraph_ConditionalBackEdge(t *testing.T) {
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	nodeB := NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", nil, nil)
	nodeC := NewDGANodeWithConfig("C", core.StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)

	// A -> B -> C (正向边)
	if err := graph.AddEdge(NewDGAEdge(nodeA, nodeB)); err != nil {
		t.Fatalf("AddEdge A->B failed: %v", err)
	}
	if err := graph.AddEdge(NewDGAEdge(nodeB, nodeC)); err != nil {
		t.Fatalf("AddEdge B->C failed: %v", err)
	}

	// C -> A: 条件回边（应被接受）
	conditionalBackEdge := NewConditionalEdge(nodeC, nodeA, "{{ iteration < 5 }}")
	if err := graph.AddEdge(conditionalBackEdge); err != nil {
		t.Fatalf("Conditional back-edge C->A should be accepted, got error: %v", err)
	}

	// 验证是循环图
	if !graph.IsCyclic() {
		t.Error("Graph should be cyclic with conditional back-edge")
	}

	// 验证有回边
	backEdges := graph.BackEdges()
	if len(backEdges) != 1 {
		t.Errorf("Expected 1 back-edge, got %d", len(backEdges))
	}
	if backEdges[0].ID() != "C->A" {
		t.Errorf("Expected back-edge C->A, got %s", backEdges[0].ID())
	}
}

// TestDGAGraph_UnconditionalCycle 测试无条件环被拒绝
func TestDGAGraph_UnconditionalCycle(t *testing.T) {
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	nodeB := NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)

	if err := graph.AddEdge(NewDGAEdge(nodeA, nodeB)); err != nil {
		t.Fatalf("AddEdge A->B failed: %v", err)
	}

	// B -> A: 无条件环（应被拒绝）
	err := graph.AddEdge(NewDGAEdge(nodeB, nodeA))
	if err != core.ErrHasCycle {
		t.Errorf("Expected ErrHasCycle for unconditional cycle, got: %v", err)
	}
}

// TestDGAGraph_BuildForwardGraph 测试构建排除回边的正向图
func TestDGAGraph_BuildForwardGraph(t *testing.T) {
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	nodeB := NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", nil, nil)
	nodeC := NewDGANodeWithConfig("C", core.StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)

	graph.AddEdge(NewDGAEdge(nodeA, nodeB))
	graph.AddEdge(NewDGAEdge(nodeB, nodeC))
	graph.AddEdge(NewConditionalEdge(nodeC, nodeA, "{{ iteration < 5 }}"))

	forwardGraph := graph.buildForwardGraph()

	// 验证正向图中 C 的出边不包含 A
	hasA := false
	for _, dest := range forwardGraph["C"] {
		if dest == "A" {
			hasA = true
		}
	}
	if hasA {
		t.Error("Forward graph should not include back-edge C->A")
	}

	// 验证正向图中 A->B, B->C 存在
	if len(forwardGraph["A"]) != 1 || forwardGraph["A"][0] != "B" {
		t.Errorf("Forward graph A should have [B], got %v", forwardGraph["A"])
	}
	if len(forwardGraph["B"]) != 1 || forwardGraph["B"][0] != "C" {
		t.Errorf("Forward graph B should have [C], got %v", forwardGraph["B"])
	}
}

// TestDGAGraph_LoopNodeSet 测试循环节点集合计算
func TestDGAGraph_LoopNodeSet(t *testing.T) {
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	nodeB := NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", nil, nil)
	nodeC := NewDGANodeWithConfig("C", core.StatusUnknown, "local", "", nil, nil)
	nodeD := NewDGANodeWithConfig("D", core.StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)
	graph.AddVertex(nodeD)

	graph.AddEdge(NewDGAEdge(nodeA, nodeB))
	graph.AddEdge(NewDGAEdge(nodeB, nodeC))
	backEdge := NewConditionalEdge(nodeC, nodeA, "{{ iteration < 5 }}")
	graph.AddEdge(backEdge)
	graph.AddEdge(NewDGAEdge(nodeC, nodeD))

	loopNodes := graph.LoopNodeSet(backEdge)

	// 循环节点应包含 A, B, C, D
	// BFS 从 target=A 出发，沿正向边可达 B, C, D，source=C 也被显式添加
	for _, id := range []string{"A", "B", "C", "D"} {
		if !loopNodes[id] {
			t.Errorf("Loop node set should contain %s", id)
		}
	}
}

// TestDGAGraph_TraversalSteps_CyclicGraph 测试循环图的 BFS 层级计算
func TestDGAGraph_TraversalSteps_CyclicGraph(t *testing.T) {
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	nodeB := NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", nil, nil)
	nodeC := NewDGANodeWithConfig("C", core.StatusUnknown, "local", "", nil, nil)
	nodeD := NewDGANodeWithConfig("D", core.StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)
	graph.AddVertex(nodeD)

	graph.AddEdge(NewDGAEdge(nodeA, nodeB))
	graph.AddEdge(NewDGAEdge(nodeB, nodeC))
	graph.AddEdge(NewConditionalEdge(nodeC, nodeA, "{{ iteration < 5 }}"))
	graph.AddEdge(NewDGAEdge(nodeC, nodeD))

	evalCtx := NewEvaluationContext()
	levels := graph.TraversalSteps(evalCtx)

	t.Logf("Levels: %v", levels)

	if len(levels) != 4 {
		t.Fatalf("Expected 4 levels, got %d", len(levels))
	}
	if levels[0][0] != "A" {
		t.Errorf("Level 0 should be [A], got %v", levels[0])
	}
	if levels[1][0] != "B" {
		t.Errorf("Level 1 should be [B], got %v", levels[1])
	}
}

// TestDGAGraph_EntryExitNodes 测试入口/出口节点跟踪
func TestDGAGraph_EntryExitNodes(t *testing.T) {
	graph := NewDGAGraph()

	graph.AddEntryNode("A")
	graph.AddEntryNode("B")
	graph.AddExitNode("D")

	entryNodes := graph.EntryNodes()
	if len(entryNodes) != 2 {
		t.Errorf("Expected 2 entry nodes, got %d", len(entryNodes))
	}

	exitNodes := graph.ExitNodes()
	if len(exitNodes) != 1 {
		t.Errorf("Expected 1 exit node, got %d", len(exitNodes))
	}
}

// TestCyclicPipeline_MaxIterations 测试循环最大迭代限制
func TestCyclicPipeline_MaxIterations(t *testing.T) {
	ctx := context.Background()

	graph := NewDGAGraph()
	nodeA := NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeB := NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo B"}}, nil)
	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)

	graph.AddEdge(NewDGAEdge(nodeA, nodeB))
	// 条件永远为 true 的回边（死循环）
	graph.AddEdge(NewConditionalEdge(nodeB, nodeA, "true"))

	// 创建 Pipeline 并设置小迭代上限
	pipeline := NewPipeline(ctx).(*PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetMaxLoopIterations(3)

	// 创建执行器提供者
	execProvider := provider.NewProvider()
	execProvider.RegisterExecutor("local", provider.ExecutorConfig{
		Type:   "local",
		Config: map[string]interface{}{},
	})
	pipeline.SetExecutorProvider(execProvider)

	err := pipeline.Run(ctx)
	if err == nil {
		t.Error("Expected error for exceeding max iterations")
	}
	t.Logf("Got expected error: %v", err)
}


// TestDGAGraph_Traversal_CyclicGraph 测试统一遍历算法在有环图上的回调遍历
// 验证 Traversal 使用 forwardGraph（排除回边）计算层级，正确遍历所有节点
func TestDGAGraph_Traversal_CyclicGraph(t *testing.T) {
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	nodeB := NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", nil, nil)
	nodeC := NewDGANodeWithConfig("C", core.StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)

	graph.AddEdge(NewDGAEdge(nodeA, nodeB))
	graph.AddEdge(NewDGAEdge(nodeB, nodeC))
	graph.AddEdge(NewConditionalEdge(nodeC, nodeA, "{{ iteration < 5 }}"))

	evalCtx := NewEvaluationContext()
	visited := []string{}
	var mu sync.Mutex

	err := graph.Traversal(context.Background(), evalCtx, func(ctx context.Context, node Node) error {
		mu.Lock()
		visited = append(visited, node.Id())
		mu.Unlock()
		return nil
	})

	if err != nil {
		t.Fatalf("Traversal on cyclic graph failed: %v", err)
	}

	if len(visited) != 3 {
		t.Errorf("Expected 3 visited nodes, got %d: %v", len(visited), visited)
	}

	// 验证所有节点都被访问
	visitedSet := make(map[string]bool)
	for _, id := range visited {
		visitedSet[id] = true
	}
	for _, id := range []string{"A", "B", "C"} {
		if !visitedSet[id] {
			t.Errorf("Node %s should be visited", id)
		}
	}
}

// TestDGAGraph_Traversal_CyclicGraph_VisitOrder 测试有环图遍历的节点访问顺序
// 验证 Traversal 按照 BFS 层级顺序执行回调（层级内可能并发，但层级间串行）
func TestDGAGraph_Traversal_CyclicGraph_VisitOrder(t *testing.T) {
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	nodeB := NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", nil, nil)
	nodeC := NewDGANodeWithConfig("C", core.StatusUnknown, "local", "", nil, nil)
	nodeD := NewDGANodeWithConfig("D", core.StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)
	graph.AddVertex(nodeD)

	// A -> B -> C -> D, 回边 C -> A
	graph.AddEdge(NewDGAEdge(nodeA, nodeB))
	graph.AddEdge(NewDGAEdge(nodeB, nodeC))
	graph.AddEdge(NewConditionalEdge(nodeC, nodeA, "{{ iteration < 3 }}"))
	graph.AddEdge(NewDGAEdge(nodeC, nodeD))

	evalCtx := NewEvaluationContext()

	// 记录每个节点的遍历序号
	visitOrder := make(map[string]int)
	var mu sync.Mutex
	counter := 0

	err := graph.Traversal(context.Background(), evalCtx, func(ctx context.Context, node Node) error {
		mu.Lock()
		visitOrder[node.Id()] = counter
		counter++
		mu.Unlock()
		return nil
	})

	if err != nil {
		t.Fatalf("Traversal failed: %v", err)
	}

	// A 必须在 B 之前，B 必须在 C 之前，C 必须在 D 之前
	if visitOrder["A"] >= visitOrder["B"] {
		t.Errorf("A (%d) should be visited before B (%d)", visitOrder["A"], visitOrder["B"])
	}
	if visitOrder["B"] >= visitOrder["C"] {
		t.Errorf("B (%d) should be visited before C (%d)", visitOrder["B"], visitOrder["C"])
	}
	if visitOrder["C"] >= visitOrder["D"] {
		t.Errorf("C (%d) should be visited before D (%d)", visitOrder["C"], visitOrder["D"])
	}
}

// TestDGAGraph_Traversal_AcyclicUnchanged 测试重写后 Traversal 在无环图上行为不变
func TestDGAGraph_Traversal_AcyclicUnchanged(t *testing.T) {
	graph := NewDGAGraph()

	node1 := NewDGANode("a", "UNKNOWN")
	node2 := NewDGANode("b", "UNKNOWN")
	node3 := NewDGANode("c", "UNKNOWN")
	node4 := NewDGANode("d", "UNKNOWN")

	graph.AddVertex(node1)
	graph.AddVertex(node2)
	graph.AddVertex(node3)
	graph.AddVertex(node4)

	// 线性图：a -> b -> c -> d
	graph.AddEdge(NewDGAEdge(node1, node2))
	graph.AddEdge(NewDGAEdge(node2, node3))
	graph.AddEdge(NewDGAEdge(node3, node4))

	evalCtx := NewEvaluationContext()
	visited := []string{}

	err := graph.Traversal(context.Background(), evalCtx, func(ctx context.Context, node Node) error {
		visited = append(visited, node.Id())
		return nil
	})

	if err != nil {
		t.Fatalf("Traversal failed: %v", err)
	}

	// 应该访问全部 4 个节点，顺序为 a, b, c, d
	if len(visited) != 4 {
		t.Errorf("Expected 4 visited nodes, got %d: %v", len(visited), visited)
	}
	expectedOrder := []string{"a", "b", "c", "d"}
	for i, id := range expectedOrder {
		if visited[i] != id {
			t.Errorf("Position %d: expected %s, got %s", i, id, visited[i])
		}
	}
}

// TestDGAGraph_Traversal_CyclicGraph_ConditionalSkipped
// 测试有环图中非回边的条件边在 Traversal 中仍然正确评估
func TestDGAGraph_Traversal_CyclicGraph_ConditionalSkipped(t *testing.T) {
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	nodeB := NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", nil, nil)
	nodeC := NewDGANodeWithConfig("C", core.StatusUnknown, "local", "", nil, nil)
	nodeD := NewDGANodeWithConfig("D", core.StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)
	graph.AddVertex(nodeD)

	graph.AddEdge(NewDGAEdge(nodeA, nodeB))
	graph.AddEdge(NewDGAEdge(nodeB, nodeC))
	// B -> D 条件边（非回边），条件不满足时应跳过
	graph.AddEdge(NewConditionalEdge(nodeB, nodeD, "{{ env == 'prod' }}"))
	// 回边 C -> A
	graph.AddEdge(NewConditionalEdge(nodeC, nodeA, "{{ iteration < 3 }}"))

	// env != prod，B->D 不应被遍历
	evalCtx := NewEvaluationContext().WithParams(map[string]any{"env": "dev"})
	visited := []string{}
	var mu sync.Mutex

	err := graph.Traversal(context.Background(), evalCtx, func(ctx context.Context, node Node) error {
		mu.Lock()
		visited = append(visited, node.Id())
		mu.Unlock()
		return nil
	})

	if err != nil {
		t.Fatalf("Traversal failed: %v", err)
	}

	visitedSet := make(map[string]bool)
	for _, id := range visited {
		visitedSet[id] = true
	}

	if visitedSet["D"] {
		t.Error("Node D should be skipped when condition env==prod is not met")
	}
	if !visitedSet["A"] || !visitedSet["B"] || !visitedSet["C"] {
		t.Errorf("Nodes A, B, C should all be visited, got: %v", visited)
	}
}

// TestDGAGraph_Traversal_CyclicGraph_ConditionalEdgeError
// 测试有环图中条件边表达式错误时 Traversal 正确返回错误
func TestDGAGraph_Traversal_CyclicGraph_ConditionalEdgeError(t *testing.T) {
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	nodeB := NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", nil, nil)
	nodeC := NewDGANodeWithConfig("C", core.StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)

	graph.AddEdge(NewDGAEdge(nodeA, nodeB))
	// B -> C 使用无效的条件表达式
	graph.AddEdge(NewConditionalEdge(nodeB, nodeC, "{{ unclosed"))
	graph.AddEdge(NewConditionalEdge(nodeC, nodeA, "{{ iteration < 5 }}"))

	evalCtx := NewEvaluationContext()
	err := graph.Traversal(context.Background(), evalCtx, func(ctx context.Context, node Node) error {
		return nil
	})

	if err == nil {
		t.Error("Expected error for invalid expression in cyclic graph conditional edge")
	}
}

// TestDGAGraph_TraversalSteps_WithEntryNodes 测试使用 entryNodes 的层级计算
func TestDGAGraph_TraversalSteps_WithEntryNodes(t *testing.T) {
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	nodeB := NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", nil, nil)
	nodeC := NewDGANodeWithConfig("C", core.StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)

	graph.AddEdge(NewDGAEdge(nodeA, nodeB))
	graph.AddEdge(NewDGAEdge(nodeB, nodeC))

	// 设置入口节点
	graph.AddEntryNode("A")

	evalCtx := NewEvaluationContext()
	levels := graph.TraversalSteps(evalCtx)

	if len(levels) != 3 {
		t.Fatalf("Expected 3 levels, got %d", len(levels))
	}
	if levels[0][0] != "A" {
		t.Errorf("Level 0 should start with entry node A, got %v", levels[0])
	}
}
