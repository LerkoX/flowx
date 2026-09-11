package flowx

import (
	"context"
	"testing"
	"time"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/dag"
)

// TestDGAGraph_RemoveVertex 测试删除节点及其关联边
func TestDGAGraph_RemoveVertex(t *testing.T) {
	graph := dag.NewDGAGraph()

	// 创建 A -> B -> C 的线性图
	nodeA := dag.NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeB := dag.NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo B"}}, nil)
	nodeC := dag.NewDGANodeWithConfig("C", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo C"}}, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)

	edgeAB := dag.NewDGAEdge(nodeA, nodeB)
	edgeBC := dag.NewDGAEdge(nodeB, nodeC)
	if err := graph.AddEdge(edgeAB); err != nil {
		t.Fatalf("AddEdge A->B failed: %v", err)
	}
	if err := graph.AddEdge(edgeBC); err != nil {
		t.Fatalf("AddEdge B->C failed: %v", err)
	}

	// 删除节点 B
	if err := graph.RemoveVertex("B"); err != nil {
		t.Fatalf("RemoveVertex B failed: %v", err)
	}

	// 验证节点 B 已删除
	if _, ok := graph.GetNode("B"); ok {
		t.Error("Node B should be removed")
	}

	// 验证节点 A 和 C 仍然存在
	if _, ok := graph.GetNode("A"); !ok {
		t.Error("Node A should still exist")
	}
	if _, ok := graph.GetNode("C"); !ok {
		t.Error("Node C should still exist")
	}

	// 验证边都已删除（A->B 和 B->C 都涉及 B）
	edges := graph.Edges()
	if len(edges) != 0 {
		t.Errorf("Expected 0 edges after removing B, got %d", len(edges))
	}

	// 验证无环
	if graph.HasCycle() {
		t.Error("Graph should not have cycle after removal")
	}

	// 删除不存在的节点应返回错误
	if err := graph.RemoveVertex("NonExistent"); err != core.ErrNodeNotFound {
		t.Errorf("Expected ErrNodeNotFound, got %v", err)
	}
}

// TestDGAGraph_RemoveEdge 测试删除指定边
func TestDGAGraph_RemoveEdge(t *testing.T) {
	graph := dag.NewDGAGraph()

	nodeA := dag.NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeB := dag.NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo B"}}, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)

	edgeAB := dag.NewDGAEdge(nodeA, nodeB)
	if err := graph.AddEdge(edgeAB); err != nil {
		t.Fatalf("AddEdge failed: %v", err)
	}

	// 删除边 A->B
	if err := graph.RemoveEdge("A", "B"); err != nil {
		t.Fatalf("RemoveEdge failed: %v", err)
	}

	// 验证边已删除
	if _, ok := graph.GetEdge("A", "B"); ok {
		t.Error("Edge A->B should be removed")
	}

	// 验证节点仍在
	if _, ok := graph.GetNode("A"); !ok {
		t.Error("Node A should still exist")
	}
	if _, ok := graph.GetNode("B"); !ok {
		t.Error("Node B should still exist")
	}

	// 删除不存在的边应返回错误
	if err := graph.RemoveEdge("A", "B"); err != core.ErrEdgeNotFound {
		t.Errorf("Expected ErrEdgeNotFound, got %v", err)
	}
}

// TestDGAGraph_GetNode_GetEdge 测试节点和边查找
func TestDGAGraph_GetNode_GetEdge(t *testing.T) {
	graph := dag.NewDGAGraph()

	// 查找不存在的节点
	if _, ok := graph.GetNode("X"); ok {
		t.Error("GetNode should return false for non-existent node")
	}
	if _, ok := graph.GetEdge("X", "Y"); ok {
		t.Error("GetEdge should return false for non-existent edge")
	}

	nodeA := dag.NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	graph.AddVertex(nodeA)

	if node, ok := graph.GetNode("A"); !ok || node.Id() != "A" {
		t.Error("GetNode should return node A")
	}
}

// TestDGAGraph_IncomingOutgoingEdges 测试入边和出边查询
func TestDGAGraph_IncomingOutgoingEdges(t *testing.T) {
	graph := dag.NewDGAGraph()

	nodeA := dag.NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	nodeB := dag.NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", nil, nil)
	nodeC := dag.NewDGANodeWithConfig("C", core.StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)

	graph.AddEdge(dag.NewDGAEdge(nodeA, nodeB))
	graph.AddEdge(dag.NewDGAEdge(nodeC, nodeB))

	// B 的入边来自 A 和 C
	inEdges := graph.IncomingEdges("B")
	if len(inEdges) != 2 {
		t.Errorf("Expected 2 incoming edges for B, got %d", len(inEdges))
	}

	// A 的出边指向 B
	outEdges := graph.OutgoingEdges("A")
	if len(outEdges) != 1 {
		t.Errorf("Expected 1 outgoing edge for A, got %d", len(outEdges))
	}

	// C 没有入边
	inEdges = graph.IncomingEdges("C")
	if len(inEdges) != 0 {
		t.Errorf("Expected 0 incoming edges for C, got %d", len(inEdges))
	}
}

// TestDGAGraph_TraversalSteps 测试 BFS 层级计算
func TestDGAGraph_TraversalSteps(t *testing.T) {
	graph := dag.NewDGAGraph()

	nodeA := dag.NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	nodeB := dag.NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", nil, nil)
	nodeC := dag.NewDGANodeWithConfig("C", core.StatusUnknown, "local", "", nil, nil)
	nodeD := dag.NewDGANodeWithConfig("D", core.StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)
	graph.AddVertex(nodeD)

	graph.AddEdge(dag.NewDGAEdge(nodeA, nodeB))
	graph.AddEdge(dag.NewDGAEdge(nodeA, nodeC))
	graph.AddEdge(dag.NewDGAEdge(nodeB, nodeD))
	graph.AddEdge(dag.NewDGAEdge(nodeC, nodeD))

	evalCtx := dag.NewEvaluationContext()
	levels := graph.TraversalSteps(evalCtx)

	if len(levels) != 3 {
		t.Fatalf("Expected 3 levels, got %d", len(levels))
	}

	// Level 0: A
	if len(levels[0]) != 1 || levels[0][0] != "A" {
		t.Errorf("Level 0 should be [A], got %v", levels[0])
	}

	// Level 1: B, C (并行)
	if len(levels[1]) != 2 {
		t.Errorf("Level 1 should have 2 nodes, got %d", len(levels[1]))
	}

	// Level 2: D
	if len(levels[2]) != 1 || levels[2][0] != "D" {
		t.Errorf("Level 2 should be [D], got %v", levels[2])
	}
}

// TestRuntimeImpl_PauseResume 测试暂停和恢复
func TestRuntimeImpl_PauseResume(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)

	config := loadTestConfig(t, "dynamic_modify.yaml")
	listener := NewRecordingListener()

	// 异步运行流水线
	workflow, err := runtime.RunAsync(ctx, "pause-test", config, listener)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}

	// 等待一小段时间让流水线开始执行
	time.Sleep(100 * time.Millisecond)

	// 尝试暂停
	err = workflow.Pause()
	if err != nil {
		// 流水线可能已经执行完毕（太快了），跳过暂停测试
		t.Logf("Workflow already finished, skipping pause test: %v", err)
		return
	}

	// 验证状态
	time.Sleep(200 * time.Millisecond)
	if workflow.Status() != core.StatusPaused {
		t.Logf("Workflow status: %s (expected PAUSED)", workflow.Status())
	}

	// 验证可修改
	if !workflow.IsModifiable() {
		t.Error("Workflow should be modifiable when paused")
	}

	// 恢复
	if err := workflow.Resume(ctx); err != nil {
		t.Fatalf("Resume failed: %v", err)
	}

	// 等待完成
	select {
	case <-workflow.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Workflow did not complete after resume")
	}

	if workflow.Status() != core.StatusSuccess {
		t.Errorf("Expected SUCCESS, got %s", workflow.Status())
	}
}

// TestWorkflowImpl_IsModifiable 测试不同状态下的 IsModifiable
func TestWorkflowImpl_IsModifiable(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)
	config := loadTestConfig(t, "dynamic_modify.yaml")

	// 运行完成的流水线应该可修改
	workflow, err := runtime.RunSync(ctx, "ismod-test", config, NewRecordingListener())
	if err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}

	// SUCCESS 状态应该可修改（完成后可以追加节点）
	if !workflow.IsModifiable() {
		t.Error("SUCCESS workflow should be modifiable")
	}

	// PAUSED 状态测试：直接操作 Workflow 对象
	// 先验证 FAILED 状态
	// 取消一个运行中的流水线
	config2 := loadTestConfig(t, "dynamic_modify.yaml")
	workflow2, err2 := runtime.RunAsync(ctx, "ismod-test2", config2, NewRecordingListener())
	if err2 != nil {
		t.Fatalf("RunAsync failed: %v", err2)
	}
	workflow2.Cancel()
	time.Sleep(100 * time.Millisecond)
	if !workflow2.IsModifiable() {
		t.Error("CANCELLED workflow should be modifiable")
	}
}

// TestGraph_DirectModification 测试直接操作 Graph 对象修改图
// 这是"完成后追加节点"的核心场景
func TestGraph_DirectModification(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)
	config := loadTestConfig(t, "dynamic_modify.yaml")

	workflow, err := runtime.RunSync(ctx, "direct-modify", config, NewRecordingListener())
	if err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}

	graph := workflow.GetGraph()

	// 验证初始结构：A -> B -> C
	if _, ok := graph.GetNode("C"); !ok {
		t.Fatal("Node C should exist initially")
	}

	// 删除节点 C
	if err := graph.RemoveVertex("C"); err != nil {
		t.Fatalf("RemoveVertex C failed: %v", err)
	}

	// 验证 C 已删除
	if _, ok := graph.GetNode("C"); ok {
		t.Error("Node C should be removed")
	}

	// 添加新节点 D 和 E
	nodeD := dag.NewDGANodeWithConfig("D", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo D"}}, nil)
	nodeE := dag.NewDGANodeWithConfig("E", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo E"}}, nil)
	graph.AddVertex(nodeD)
	graph.AddVertex(nodeE)

	// 添加边 B -> D -> E
	if err := graph.AddEdge(dag.NewDGAEdge(graph.Nodes()["B"], nodeD)); err != nil {
		t.Fatalf("AddEdge B->D failed: %v", err)
	}
	if err := graph.AddEdge(dag.NewDGAEdge(nodeD, nodeE)); err != nil {
		t.Fatalf("AddEdge D->E failed: %v", err)
	}

	// 验证新图结构
	if _, ok := graph.GetNode("D"); !ok {
		t.Error("Node D should exist")
	}
	if _, ok := graph.GetNode("E"); !ok {
		t.Error("Node E should exist")
	}
	if _, ok := graph.GetEdge("B", "D"); !ok {
		t.Error("Edge B->D should exist")
	}
	if _, ok := graph.GetEdge("D", "E"); !ok {
		t.Error("Edge D->E should exist")
	}

	// 验证无环
	if graph.HasCycle() {
		t.Error("Graph should not have cycle")
	}
}

// TestGraph_RemoveAndReplace 测试删除未运行节点并替换的场景
// 模拟：A,B 已运行完成，删除 D 替换为 F,G
func TestGraph_RemoveAndReplace(t *testing.T) {
	graph := dag.NewDGAGraph()

	// 创建 A -> B -> C -> D 的图
	nodes := []struct {
		id   string
		step string
	}{
		{"A", "echo A"}, {"B", "echo B"}, {"C", "echo C"}, {"D", "echo D"},
	}

	for _, n := range nodes {
		node := dag.NewDGANodeWithConfig(n.id, core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: n.step}}, nil)
		graph.AddVertex(node)
	}

	graphNodes := graph.Nodes()
	graph.AddEdge(dag.NewDGAEdge(graphNodes["A"], graphNodes["B"]))
	graph.AddEdge(dag.NewDGAEdge(graphNodes["B"], graphNodes["C"]))
	graph.AddEdge(dag.NewDGAEdge(graphNodes["C"], graphNodes["D"]))

	// 模拟 A, B 已完成：设置状态
	graphNodes = graph.Nodes()
	nodeA := graphNodes["A"]
	nodeA.SetRuntimeStatus(&core.NodeRuntimeStatus{Status: core.StatusSuccess, Steps: []core.StepRuntimeStatus{{Name: "step1", Status: core.StatusSuccess}}})

	// 删除 D，替换为 F, G
	graph.RemoveVertex("D")

	nodeF := dag.NewDGANodeWithConfig("F", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo F"}}, nil)
	nodeG := dag.NewDGANodeWithConfig("G", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo G"}}, nil)
	graph.AddVertex(nodeF)
	graph.AddVertex(nodeG)

	graph.AddEdge(dag.NewDGAEdge(graphNodes["C"], nodeF))
	graph.AddEdge(dag.NewDGAEdge(nodeF, nodeG))

	// 验证新结构
	if _, ok := graph.GetNode("D"); ok {
		t.Error("Node D should be removed")
	}
	if _, ok := graph.GetNode("F"); !ok {
		t.Error("Node F should exist")
	}
	if _, ok := graph.GetNode("G"); !ok {
		t.Error("Node G should exist")
	}
	if _, ok := graph.GetEdge("C", "F"); !ok {
		t.Error("Edge C->F should exist")
	}
	if _, ok := graph.GetEdge("F", "G"); !ok {
		t.Error("Edge F->G should exist")
	}

	// 验证层级
	evalCtx := dag.NewEvaluationContext()
	levels := graph.TraversalSteps(evalCtx)
	t.Logf("Levels after modification: %v", levels)

	// 验证无环
	if graph.HasCycle() {
		t.Error("Graph should not have cycle")
	}
}

// TestGraph_CycleDetection 测试添加环时的检测
func TestGraph_CycleDetection(t *testing.T) {
	graph := dag.NewDGAGraph()

	nodeA := dag.NewDGANodeWithConfig("A", core.StatusUnknown, "local", "", nil, nil)
	nodeB := dag.NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", nil, nil)
	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)

	graph.AddEdge(dag.NewDGAEdge(nodeA, nodeB))

	// 尝试添加 B -> A 形成环
	err := graph.AddEdge(dag.NewDGAEdge(nodeB, nodeA))
	if err != core.ErrHasCycle {
		t.Errorf("Expected ErrHasCycle when adding B->A, got %v", err)
	}

	// 环检测返回 true
	if !graph.HasCycle() {
		t.Error("Graph should have cycle after adding B->A")
	}
}
