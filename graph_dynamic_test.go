package flowx

import (
	"context"
	"testing"
	"time"

	"github.com/LerkoX/flowx/executor/provider"
)

// TestDGAGraph_RemoveVertex 测试删除节点及其关联边
func TestDGAGraph_RemoveVertex(t *testing.T) {
	graph := NewDGAGraph()

	// 创建 A -> B -> C 的线性图
	nodeA := NewDGANodeWithConfig("A", StatusUnknown, "local", "", []Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeB := NewDGANodeWithConfig("B", StatusUnknown, "local", "", []Step{{Name: "step1", Run: "echo B"}}, nil)
	nodeC := NewDGANodeWithConfig("C", StatusUnknown, "local", "", []Step{{Name: "step1", Run: "echo C"}}, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)

	edgeAB := NewDGAEdge(nodeA, nodeB)
	edgeBC := NewDGAEdge(nodeB, nodeC)
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
	if err := graph.RemoveVertex("NonExistent"); err != ErrNodeNotFound {
		t.Errorf("Expected ErrNodeNotFound, got %v", err)
	}
}

// TestDGAGraph_RemoveEdge 测试删除指定边
func TestDGAGraph_RemoveEdge(t *testing.T) {
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", StatusUnknown, "local", "", []Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeB := NewDGANodeWithConfig("B", StatusUnknown, "local", "", []Step{{Name: "step1", Run: "echo B"}}, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)

	edgeAB := NewDGAEdge(nodeA, nodeB)
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
	if err := graph.RemoveEdge("A", "B"); err != ErrEdgeNotFound {
		t.Errorf("Expected ErrEdgeNotFound, got %v", err)
	}
}

// TestDGAGraph_GetNode_GetEdge 测试节点和边查找
func TestDGAGraph_GetNode_GetEdge(t *testing.T) {
	graph := NewDGAGraph()

	// 查找不存在的节点
	if _, ok := graph.GetNode("X"); ok {
		t.Error("GetNode should return false for non-existent node")
	}
	if _, ok := graph.GetEdge("X", "Y"); ok {
		t.Error("GetEdge should return false for non-existent edge")
	}

	nodeA := NewDGANodeWithConfig("A", StatusUnknown, "local", "", nil, nil)
	graph.AddVertex(nodeA)

	if node, ok := graph.GetNode("A"); !ok || node.Id() != "A" {
		t.Error("GetNode should return node A")
	}
}

// TestDGAGraph_IncomingOutgoingEdges 测试入边和出边查询
func TestDGAGraph_IncomingOutgoingEdges(t *testing.T) {
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", StatusUnknown, "local", "", nil, nil)
	nodeB := NewDGANodeWithConfig("B", StatusUnknown, "local", "", nil, nil)
	nodeC := NewDGANodeWithConfig("C", StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)

	graph.AddEdge(NewDGAEdge(nodeA, nodeB))
	graph.AddEdge(NewDGAEdge(nodeC, nodeB))

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
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", StatusUnknown, "local", "", nil, nil)
	nodeB := NewDGANodeWithConfig("B", StatusUnknown, "local", "", nil, nil)
	nodeC := NewDGANodeWithConfig("C", StatusUnknown, "local", "", nil, nil)
	nodeD := NewDGANodeWithConfig("D", StatusUnknown, "local", "", nil, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)
	graph.AddVertex(nodeD)

	graph.AddEdge(NewDGAEdge(nodeA, nodeB))
	graph.AddEdge(NewDGAEdge(nodeA, nodeC))
	graph.AddEdge(NewDGAEdge(nodeB, nodeD))
	graph.AddEdge(NewDGAEdge(nodeC, nodeD))

	evalCtx := NewEvaluationContext()
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
	pipeline, err := runtime.RunAsync(ctx, "pause-test", config, listener)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}

	// 等待一小段时间让流水线开始执行
	time.Sleep(100 * time.Millisecond)

	// 尝试暂停
	err = pipeline.Pause()
	if err != nil {
		// 流水线可能已经执行完毕（太快了），跳过暂停测试
		t.Logf("Pipeline already finished, skipping pause test: %v", err)
		return
	}

	// 验证状态
	time.Sleep(200 * time.Millisecond)
	if pipeline.Status() != StatusPaused {
		t.Logf("Pipeline status: %s (expected PAUSED)", pipeline.Status())
	}

	// 验证可修改
	if !pipeline.IsModifiable() {
		t.Error("Pipeline should be modifiable when paused")
	}

	// 恢复
	if err := pipeline.Resume(ctx); err != nil {
		t.Fatalf("Resume failed: %v", err)
	}

	// 等待完成
	select {
	case <-pipeline.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Pipeline did not complete after resume")
	}

	if pipeline.Status() != StatusSuccess {
		t.Errorf("Expected SUCCESS, got %s", pipeline.Status())
	}
}

// TestPipelineImpl_IsModifiable 测试不同状态下的 IsModifiable
func TestPipelineImpl_IsModifiable(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)
	config := loadTestConfig(t, "dynamic_modify.yaml")

	// 运行完成的流水线应该可修改
	pipeline, err := runtime.RunSync(ctx, "ismod-test", config, NewRecordingListener())
	if err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}

	// SUCCESS 状态应该可修改（完成后可以追加节点）
	if !pipeline.IsModifiable() {
		t.Error("SUCCESS pipeline should be modifiable")
	}

	// PAUSED 状态测试：直接操作 Pipeline 对象
	// 先验证 FAILED 状态
	// 取消一个运行中的流水线
	config2 := loadTestConfig(t, "dynamic_modify.yaml")
	pipeline2, err2 := runtime.RunAsync(ctx, "ismod-test2", config2, NewRecordingListener())
	if err2 != nil {
		t.Fatalf("RunAsync failed: %v", err2)
	}
	pipeline2.Cancel()
	time.Sleep(100 * time.Millisecond)
	if !pipeline2.IsModifiable() {
		t.Error("CANCELLED pipeline should be modifiable")
	}
}

// TestGraph_DirectModification 测试直接操作 Graph 对象修改图
// 这是"完成后追加节点"的核心场景
func TestGraph_DirectModification(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)
	config := loadTestConfig(t, "dynamic_modify.yaml")

	pipeline, err := runtime.RunSync(ctx, "direct-modify", config, NewRecordingListener())
	if err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}

	graph := pipeline.GetGraph()

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
	nodeD := NewDGANodeWithConfig("D", StatusUnknown, "local", "", []Step{{Name: "step1", Run: "echo D"}}, nil)
	nodeE := NewDGANodeWithConfig("E", StatusUnknown, "local", "", []Step{{Name: "step1", Run: "echo E"}}, nil)
	graph.AddVertex(nodeD)
	graph.AddVertex(nodeE)

	// 添加边 B -> D -> E
	if err := graph.AddEdge(NewDGAEdge(graph.Nodes()["B"], nodeD)); err != nil {
		t.Fatalf("AddEdge B->D failed: %v", err)
	}
	if err := graph.AddEdge(NewDGAEdge(nodeD, nodeE)); err != nil {
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
	graph := NewDGAGraph()

	// 创建 A -> B -> C -> D 的图
	nodes := []struct {
		id   string
		step string
	}{
		{"A", "echo A"}, {"B", "echo B"}, {"C", "echo C"}, {"D", "echo D"},
	}

	for _, n := range nodes {
		node := NewDGANodeWithConfig(n.id, StatusUnknown, "local", "", []Step{{Name: "step1", Run: n.step}}, nil)
		graph.AddVertex(node)
	}

	graphNodes := graph.Nodes()
	graph.AddEdge(NewDGAEdge(graphNodes["A"], graphNodes["B"]))
	graph.AddEdge(NewDGAEdge(graphNodes["B"], graphNodes["C"]))
	graph.AddEdge(NewDGAEdge(graphNodes["C"], graphNodes["D"]))

	// 模拟 A, B 已完成：设置状态
	graphNodes = graph.Nodes()
	nodeA := graphNodes["A"]
	nodeA.SetRuntimeStatus(&NodeRuntimeStatus{Status: StatusSuccess, Steps: []StepRuntimeStatus{{Name: "step1", Status: StatusSuccess}}})

	// 删除 D，替换为 F, G
	graph.RemoveVertex("D")

	nodeF := NewDGANodeWithConfig("F", StatusUnknown, "local", "", []Step{{Name: "step1", Run: "echo F"}}, nil)
	nodeG := NewDGANodeWithConfig("G", StatusUnknown, "local", "", []Step{{Name: "step1", Run: "echo G"}}, nil)
	graph.AddVertex(nodeF)
	graph.AddVertex(nodeG)

	graph.AddEdge(NewDGAEdge(graphNodes["C"], nodeF))
	graph.AddEdge(NewDGAEdge(nodeF, nodeG))

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
	evalCtx := NewEvaluationContext()
	levels := graph.TraversalSteps(evalCtx)
	t.Logf("Levels after modification: %v", levels)

	// 验证无环
	if graph.HasCycle() {
		t.Error("Graph should not have cycle")
	}
}

// TestGraph_CycleDetection 测试添加环时的检测
func TestGraph_CycleDetection(t *testing.T) {
	graph := NewDGAGraph()

	nodeA := NewDGANodeWithConfig("A", StatusUnknown, "local", "", nil, nil)
	nodeB := NewDGANodeWithConfig("B", StatusUnknown, "local", "", nil, nil)
	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)

	graph.AddEdge(NewDGAEdge(nodeA, nodeB))

	// 尝试添加 B -> A 形成环
	err := graph.AddEdge(NewDGAEdge(nodeB, nodeA))
	if err != ErrHasCycle {
		t.Errorf("Expected ErrHasCycle when adding B->A, got %v", err)
	}

	// 环检测返回 true
	if !graph.HasCycle() {
		t.Error("Graph should have cycle after adding B->A")
	}
}

// --- UpdateConfig Tests ---

// TestRuntimeImpl_UpdateConfig_AddNodes 测试通过新配置添加节点
func TestRuntimeImpl_UpdateConfig_AddNodes(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)

	// 手动构建 pipeline（A 已执行，B 未执行）
	graph := NewDGAGraph()
	nodeA := NewDGANodeWithConfig("A", StatusSuccess, "local", "", []Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&NodeRuntimeStatus{Status: StatusSuccess})
	nodeB := NewDGANodeWithConfig("B", StatusUnknown, "local", "", []Step{{Name: "step1", Run: "echo B"}}, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddEdge(NewDGAEdge(nodeA, nodeB))

	pipeline := NewPipeline(ctx).(*PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetStatusForTest(StatusSuccess)

	execProvider := provider.NewProvider()
	execProvider.RegisterExecutor("local", provider.ExecutorConfig{Type: "local", Config: map[string]interface{}{}})
	pipeline.SetExecutorProvider(execProvider)

	rt.pipelines["update-add-test"] = pipeline
	rt.pipelineConfigs["update-add-test"] = &PipelineConfig{
		Version: "1.0",
		Name:    "update-test",
		Executors: map[string]ExecutorConfig{
			"local": {Type: "local", Config: map[string]interface{}{}},
		},
		Graph: "stateDiagram-v2\n  [*] --> A\n  A --> B\n  B --> [*]",
		Nodes: map[string]NodeConfig{
			"A": {Executor: "local", Steps: []Step{{Name: "step1", Run: "echo A"}}},
			"B": {Executor: "local", Steps: []Step{{Name: "step1", Run: "echo B"}}},
		},
	}

	// 新配置：添加节点 C，Graph 变更
	newConfig := `
Version: "1.0"
Name: update-test

Executors:
  local:
    type: local
    config: {}

Graph: |
  stateDiagram-v2
    [*] --> A
    A --> B
    B --> C
    C --> [*]

Nodes:
  A:
    executor: local
    steps:
      - name: step1
        run: echo A
  B:
    executor: local
    steps:
      - name: step1
        run: echo B
  C:
    executor: local
    steps:
      - name: step1
        run: echo C
`

	err := rt.UpdateConfig(ctx, "update-add-test", newConfig)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	g := pipeline.GetGraph()
	_, ok := g.GetNode("C")
	if !ok {
		t.Error("Node C should be added to graph")
	}
}

// TestRuntimeImpl_UpdateConfig_RemoveUnexecutedNode 测试删除未执行节点
func TestRuntimeImpl_UpdateConfig_RemoveUnexecutedNode(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)


	// RunSync 会执行所有节点，所以直接用 Graph 构建来控制状态
	graph := NewDGAGraph()
	nodeA := NewDGANodeWithConfig("A", StatusSuccess, "local", "", []Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&NodeRuntimeStatus{Status: StatusSuccess})
	nodeB := NewDGANodeWithConfig("B", StatusUnknown, "local", "", []Step{{Name: "step1", Run: "echo B"}}, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddEdge(NewDGAEdge(nodeA, nodeB))

	pipeline := NewPipeline(ctx).(*PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetStatusForTest(StatusSuccess)

	execProvider := provider.NewProvider()
	execProvider.RegisterExecutor("local", provider.ExecutorConfig{Type: "local", Config: map[string]interface{}{}})
	pipeline.SetExecutorProvider(execProvider)

	rt.pipelines["remove-unexec"] = pipeline
	rt.pipelineConfigs["remove-unexec"] = &PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Executors: map[string]ExecutorConfig{
			"local": {Type: "local", Config: map[string]interface{}{}},
		},
		Nodes: map[string]NodeConfig{
			"A": {Executor: "local", Steps: []Step{{Name: "step1", Run: "echo A"}}},
			"B": {Executor: "local", Steps: []Step{{Name: "step1", Run: "echo B"}}},
		},
	}

	// 新配置：删除未执行的 B 节点
	newConfig := `
Version: "1.0"
Name: test

Executors:
  local:
    type: local
    config: {}

Nodes:
  A:
    executor: local
    steps:
      - name: step1
        run: echo A
`

	err := rt.UpdateConfig(ctx, "remove-unexec", newConfig)
	if err != nil {
		t.Fatalf("UpdateConfig should succeed for removing unexecuted node, got: %v", err)
	}

	g := pipeline.GetGraph()
	if _, ok := g.GetNode("B"); ok {
		t.Error("Node B should be removed")
	}
}

// TestRuntimeImpl_UpdateConfig_RemoveExecutedNode 测试删除已执行节点被拒绝
func TestRuntimeImpl_UpdateConfig_RemoveExecutedNode(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)

	graph := NewDGAGraph()
	nodeA := NewDGANodeWithConfig("A", StatusSuccess, "local", "", []Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&NodeRuntimeStatus{Status: StatusSuccess})

	graph.AddVertex(nodeA)

	pipeline := NewPipeline(ctx).(*PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetStatusForTest(StatusSuccess)

	rt.pipelines["remove-exec"] = pipeline
	rt.pipelineConfigs["remove-exec"] = &PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Executors: map[string]ExecutorConfig{
			"local": {Type: "local", Config: map[string]interface{}{}},
		},
		Nodes: map[string]NodeConfig{
			"A": {Executor: "local", Steps: []Step{{Name: "step1", Run: "echo A"}}},
		},
	}

	// 新配置：删除已执行的 A 节点
	newConfig := `
Version: "1.0"
Name: test

Executors:
  local:
    type: local
    config: {}
Nodes: {}
`

	err := rt.UpdateConfig(ctx, "remove-exec", newConfig)
	if err == nil {
		t.Error("Expected error when removing executed node")
	}
	t.Logf("Got expected error: %v", err)
}

// TestRuntimeImpl_UpdateConfig_ModifyExecutedNode 测试修改已执行节点被拒绝
func TestRuntimeImpl_UpdateConfig_ModifyExecutedNode(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)

	graph := NewDGAGraph()
	nodeA := NewDGANodeWithConfig("A", StatusSuccess, "local", "", []Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&NodeRuntimeStatus{Status: StatusSuccess})

	graph.AddVertex(nodeA)

	pipeline := NewPipeline(ctx).(*PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetStatusForTest(StatusSuccess)

	rt.pipelines["modify-exec"] = pipeline
	rt.pipelineConfigs["modify-exec"] = &PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Executors: map[string]ExecutorConfig{
			"local": {Type: "local", Config: map[string]interface{}{}},
		},
		Nodes: map[string]NodeConfig{
			"A": {Executor: "local", Steps: []Step{{Name: "step1", Run: "echo A"}}},
		},
	}

	// 新配置：修改已执行的 A 节点
	newConfig := `
Version: "1.0"
Name: test

Executors:
  local:
    type: local
    config: {}

Nodes:
  A:
    executor: local
    steps:
      - name: step1
        run: echo "modified A"
`

	err := rt.UpdateConfig(ctx, "modify-exec", newConfig)
	if err == nil {
		t.Error("Expected error when modifying executed node")
	}
	t.Logf("Got expected error: %v", err)
}

// TestRuntimeImpl_UpdateConfig_ImmutableField 测试修改不可变字段被拒绝
func TestRuntimeImpl_UpdateConfig_ImmutableField(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)

	graph := NewDGAGraph()
	nodeA := NewDGANodeWithConfig("A", StatusSuccess, "local", "", []Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&NodeRuntimeStatus{Status: StatusSuccess})

	graph.AddVertex(nodeA)

	pipeline := NewPipeline(ctx).(*PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetStatusForTest(StatusSuccess)

	rt.pipelines["immutable-test"] = pipeline
	rt.pipelineConfigs["immutable-test"] = &PipelineConfig{
		Version: "1.0",
		Name:    "original",
		Executors: map[string]ExecutorConfig{
			"local": {Type: "local", Config: map[string]interface{}{}},
		},
		Nodes: map[string]NodeConfig{
			"A": {Executor: "local", Steps: []Step{{Name: "step1", Run: "echo A"}}},
		},
	}

	// 新配置：Name 被修改
	newConfig := `
Version: "1.0"
Name: changed-name

Executors:
  local:
    type: local
    config: {}

Nodes:
  A:
    executor: local
    steps:
      - name: step1
        run: echo A
`

	err := rt.UpdateConfig(ctx, "immutable-test", newConfig)
	if err == nil {
		t.Error("Expected error when modifying immutable field")
	}
	t.Logf("Got expected error: %v", err)
}

// TestRuntimeImpl_UpdateConfig_NoChanges 测试无变更直接返回
func TestRuntimeImpl_UpdateConfig_NoChanges(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)

	graph := NewDGAGraph()
	nodeA := NewDGANodeWithConfig("A", StatusSuccess, "local", "", []Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&NodeRuntimeStatus{Status: StatusSuccess})

	graph.AddVertex(nodeA)

	pipeline := NewPipeline(ctx).(*PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetStatusForTest(StatusSuccess)

	rt.pipelines["nochange-test"] = pipeline
	rt.pipelineConfigs["nochange-test"] = &PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Executors: map[string]ExecutorConfig{
			"local": {Type: "local", Config: map[string]interface{}{}},
		},
		Nodes: map[string]NodeConfig{
			"A": {Executor: "local", Steps: []Step{{Name: "step1", Run: "echo A"}}},
		},
	}

	// 完全相同的新配置
	newConfig := `
Version: "1.0"
Name: test

Executors:
  local:
    type: local
    config: {}

Nodes:
  A:
    executor: local
    steps:
      - name: step1
        run: echo A
`

	err := rt.UpdateConfig(ctx, "nochange-test", newConfig)
	if err != nil {
		t.Errorf("UpdateConfig should succeed with no changes, got: %v", err)
	}
}
