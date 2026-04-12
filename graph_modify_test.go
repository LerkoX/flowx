package flowx

import (
	"context"
	"testing"

	"github.com/LerkoX/flowx/executor/provider"
)

// helperSetupModifiablePipeline 创建一个处于 PAUSED 状态的 pipeline 并注册到 runtime
func helperSetupModifiablePipeline(t *testing.T, rt *RuntimeImpl, id string) (Pipeline, Graph) {
	t.Helper()

	graph := NewDGAGraph()
	nodeA := NewDGANodeWithConfig("A", StatusSuccess, "local", "", []Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&NodeRuntimeStatus{Status: StatusSuccess})
	nodeB := NewDGANodeWithConfig("B", StatusUnknown, "local", "", []Step{{Name: "step1", Run: "echo B"}}, nil)
	nodeC := NewDGANodeWithConfig("C", StatusUnknown, "local", "", []Step{{Name: "step1", Run: "echo C"}}, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)
	graph.AddEdge(NewDGAEdge(nodeA, nodeB))
	graph.AddEdge(NewDGAEdge(nodeB, nodeC))

	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.status = StatusPaused // 设置为可修改状态

	execProvider := provider.NewProvider()
	execProvider.RegisterExecutor("local", provider.ExecutorConfig{Type: "local", Config: map[string]any{}})
	pipeline.SetExecutorProvider(execProvider)

	rt.pipelines[id] = pipeline
	rt.pipelineConfigs[id] = &PipelineConfig{
		Version: "1.0",
		Name:    "modify-test",
		Executors: map[string]ExecutorConfig{
			"local": {Type: "local", Config: map[string]any{}},
		},
		Graph: "stateDiagram-v2\n  [*] --> A\n  A --> B\n  B --> C\n  C --> [*]",
		Nodes: map[string]NodeConfig{
			"A": {Name: "A", Executor: "local", Steps: []Step{{Name: "step1", Run: "echo A"}}},
			"B": {Name: "B", Executor: "local", Steps: []Step{{Name: "step1", Run: "echo B"}}},
			"C": {Name: "C", Executor: "local", Steps: []Step{{Name: "step1", Run: "echo C"}}},
		},
	}

	return pipeline, graph
}

func TestModifyGraph_AddNode(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	pipeline, _ := helperSetupModifiablePipeline(t, rt, "add-node")

	mods := GraphModifications{
		AddNodes: []NodeConfig{
			{Name: "D", Executor: "local", Steps: []Step{{Name: "step1", Run: "echo D"}}},
		},
	}

	err := rt.ModifyGraph(ctx, "add-node", mods)
	if err != nil {
		t.Fatalf("ModifyGraph() error = %v", err)
	}

	graph := pipeline.GetGraph()
	if _, ok := graph.GetNode("D"); !ok {
		t.Error("Node D should exist after ModifyGraph")
	}
}

func TestModifyGraph_RemoveNode(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	pipeline, _ := helperSetupModifiablePipeline(t, rt, "remove-node")

	mods := GraphModifications{
		RemoveNodes: []string{"C"},
	}

	err := rt.ModifyGraph(ctx, "remove-node", mods)
	if err != nil {
		t.Fatalf("ModifyGraph() error = %v", err)
	}

	graph := pipeline.GetGraph()
	if _, ok := graph.GetNode("C"); ok {
		t.Error("Node C should be removed")
	}
}

func TestModifyGraph_AddEdge(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	pipeline, _ := helperSetupModifiablePipeline(t, rt, "add-edge")

	mods := GraphModifications{
		AddEdges: []EdgeModification{
			{Source: "A", Target: "C"},
		},
	}

	err := rt.ModifyGraph(ctx, "add-edge", mods)
	if err != nil {
		t.Fatalf("ModifyGraph() error = %v", err)
	}

	graph := pipeline.GetGraph()
	if _, ok := graph.GetEdge("A", "C"); !ok {
		t.Error("Edge A->C should exist")
	}
}

func TestModifyGraph_RemoveEdge(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	pipeline, _ := helperSetupModifiablePipeline(t, rt, "remove-edge")

	mods := GraphModifications{
		RemoveEdges: []EdgeRemoval{
			{Source: "B", Target: "C"},
		},
	}

	err := rt.ModifyGraph(ctx, "remove-edge", mods)
	if err != nil {
		t.Fatalf("ModifyGraph() error = %v", err)
	}

	graph := pipeline.GetGraph()
	if _, ok := graph.GetEdge("B", "C"); ok {
		t.Error("Edge B->C should be removed")
	}
}

func TestModifyGraph_ConditionalBackEdge(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	pipeline, _ := helperSetupModifiablePipeline(t, rt, "cond-back-edge")

	mods := GraphModifications{
		AddEdges: []EdgeModification{
			{Source: "C", Target: "A", Expression: "{{ iteration < 3 }}"},
		},
	}

	err := rt.ModifyGraph(ctx, "cond-back-edge", mods)
	if err != nil {
		t.Fatalf("ModifyGraph() with conditional back edge error = %v", err)
	}

	graph := pipeline.GetGraph()
	if _, ok := graph.GetEdge("C", "A"); !ok {
		t.Error("Conditional edge C->A should exist")
	}
}

func TestModifyGraph_UnconditionalCycle(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	_, _ = helperSetupModifiablePipeline(t, rt, "uncond-cycle")

	mods := GraphModifications{
		AddEdges: []EdgeModification{
			{Source: "C", Target: "A"},
		},
	}

	err := rt.ModifyGraph(ctx, "uncond-cycle", mods)
	if err == nil {
		t.Fatal("Expected error for unconditional cycle")
	}
	// 错误被 wrap 为 "failed to add edge C->A: has cycle"
	if err.Error() != "has cycle" && err.Error() != "failed to add edge C->A: has cycle" {
		t.Errorf("Error = %v, want cycle-related error", err)
	}
}

func TestModifyGraph_NotFound(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)

	err := rt.ModifyGraph(ctx, "nonexistent", GraphModifications{})
	if err == nil {
		t.Error("Expected error for non-existent pipeline")
	}
}

func TestModifyGraph_RollbackOnEdgeError(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	pipeline, _ := helperSetupModifiablePipeline(t, rt, "rollback-edge")

	graph := pipeline.GetGraph()

	// 添加到不存在目标的边应该失败
	mods := GraphModifications{
		AddEdges: []EdgeModification{
			{Source: "A", Target: "NonExistent"},
		},
	}

	err := rt.ModifyGraph(ctx, "rollback-edge", mods)
	if err == nil {
		t.Error("Expected error for non-existent target node")
	}

	// 回滚：原始节点/边不应该受影响
	if _, ok := graph.GetNode("A"); !ok {
		t.Error("Node A should still exist after rollback")
	}
	if _, ok := graph.GetNode("B"); !ok {
		t.Error("Node B should still exist after rollback")
	}
	if _, ok := graph.GetEdge("A", "B"); !ok {
		t.Error("Edge A->B should still exist after rollback")
	}
}

func TestModifyGraph_ComplexModification(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	pipeline, _ := helperSetupModifiablePipeline(t, rt, "complex-mod")

	mods := GraphModifications{
		RemoveEdges: []EdgeRemoval{
			{Source: "B", Target: "C"},
		},
		AddNodes: []NodeConfig{
			{Name: "D", Executor: "local", Steps: []Step{{Name: "step1", Run: "echo D"}}},
		},
		AddEdges: []EdgeModification{
			{Source: "B", Target: "D"},
		},
	}

	err := rt.ModifyGraph(ctx, "complex-mod", mods)
	if err != nil {
		t.Fatalf("ModifyGraph() error = %v", err)
	}

	graph := pipeline.GetGraph()
	if _, ok := graph.GetEdge("B", "C"); ok {
		t.Error("Edge B->C should be removed")
	}
	if _, ok := graph.GetNode("D"); !ok {
		t.Error("Node D should exist")
	}
	if _, ok := graph.GetEdge("B", "D"); !ok {
		t.Error("Edge B->D should exist")
	}
}
