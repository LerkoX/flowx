package flowx

import (
	"context"
	"testing"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/dag"
	"github.com/LerkoX/flowx/executor/provider"
)

// helperSetupModifiableWorkflow 创建一个处于 PAUSED 状态的 workflow 并注册到 runtime
func helperSetupModifiableWorkflow(t *testing.T, rt *RuntimeImpl, id string) (dag.Workflow, dag.Graph) {
	t.Helper()

	graph := dag.NewDGAGraph()
	nodeA := dag.NewDGANodeWithConfig("A", core.StatusSuccess, "local", "", []core.Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&core.NodeRuntimeStatus{Status: core.StatusSuccess})
	nodeB := dag.NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo B"}}, nil)
	nodeC := dag.NewDGANodeWithConfig("C", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo C"}}, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddVertex(nodeC)
	graph.AddEdge(dag.NewDGAEdge(nodeA, nodeB))
	graph.AddEdge(dag.NewDGAEdge(nodeB, nodeC))

	workflow := dag.NewWorkflow(context.Background()).(*dag.WorkflowImpl)
	workflow.SetGraph(graph)
	workflow.SetStatusForTest(core.StatusPaused) // 设置为可修改状态

	execProvider := provider.NewProvider()
	execProvider.RegisterExecutor("local", provider.ExecutorConfig{Type: "local", Config: map[string]any{}})
	workflow.SetExecutorProvider(execProvider)

	rt.workflows[id] = workflow
	rt.workflowConfigs[id] = &core.WorkflowConfig{
		Version: "1.0",
		Name:    "modify-test",
		Executors: map[string]core.ExecutorConfig{
			"local": {Type: "local", Config: map[string]any{}},
		},
		Graph: "stateDiagram-v2\n  [*] --> A\n  A --> B\n  B --> C\n  C --> [*]",
		Nodes: map[string]core.NodeConfig{
			"A": {Name: "A", Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo A"}}},
			"B": {Name: "B", Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo B"}}},
			"C": {Name: "C", Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo C"}}},
		},
	}

	return workflow, graph
}

func TestModifyGraph_AddNode(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	workflow, _ := helperSetupModifiableWorkflow(t, rt, "add-node")

	mods := dag.GraphModifications{
		AddNodes: []core.NodeConfig{
			{Name: "D", Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo D"}}},
		},
	}

	err := rt.ModifyGraph(ctx, "add-node", mods)
	if err != nil {
		t.Fatalf("ModifyGraph() error = %v", err)
	}

	graph := workflow.GetGraph()
	if _, ok := graph.GetNode("D"); !ok {
		t.Error("Node D should exist after ModifyGraph")
	}
}

func TestModifyGraph_RemoveNode(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	workflow, _ := helperSetupModifiableWorkflow(t, rt, "remove-node")

	mods := dag.GraphModifications{
		RemoveNodes: []string{"C"},
	}

	err := rt.ModifyGraph(ctx, "remove-node", mods)
	if err != nil {
		t.Fatalf("ModifyGraph() error = %v", err)
	}

	graph := workflow.GetGraph()
	if _, ok := graph.GetNode("C"); ok {
		t.Error("Node C should be removed")
	}
}

func TestModifyGraph_AddEdge(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	workflow, _ := helperSetupModifiableWorkflow(t, rt, "add-edge")

	mods := dag.GraphModifications{
		AddEdges: []dag.EdgeModification{
			{Source: "A", Target: "C"},
		},
	}

	err := rt.ModifyGraph(ctx, "add-edge", mods)
	if err != nil {
		t.Fatalf("ModifyGraph() error = %v", err)
	}

	graph := workflow.GetGraph()
	if _, ok := graph.GetEdge("A", "C"); !ok {
		t.Error("Edge A->C should exist")
	}
}

func TestModifyGraph_RemoveEdge(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	workflow, _ := helperSetupModifiableWorkflow(t, rt, "remove-edge")

	mods := dag.GraphModifications{
		RemoveEdges: []dag.EdgeRemoval{
			{Source: "B", Target: "C"},
		},
	}

	err := rt.ModifyGraph(ctx, "remove-edge", mods)
	if err != nil {
		t.Fatalf("ModifyGraph() error = %v", err)
	}

	graph := workflow.GetGraph()
	if _, ok := graph.GetEdge("B", "C"); ok {
		t.Error("Edge B->C should be removed")
	}
}

func TestModifyGraph_ConditionalBackEdge(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	workflow, _ := helperSetupModifiableWorkflow(t, rt, "cond-back-edge")

	mods := dag.GraphModifications{
		AddEdges: []dag.EdgeModification{
			{Source: "C", Target: "A", Expression: "{{ iteration < 3 }}"},
		},
	}

	err := rt.ModifyGraph(ctx, "cond-back-edge", mods)
	if err != nil {
		t.Fatalf("ModifyGraph() with conditional back edge error = %v", err)
	}

	graph := workflow.GetGraph()
	if _, ok := graph.GetEdge("C", "A"); !ok {
		t.Error("Conditional edge C->A should exist")
	}
}

func TestModifyGraph_UnconditionalCycle(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	_, _ = helperSetupModifiableWorkflow(t, rt, "uncond-cycle")

	mods := dag.GraphModifications{
		AddEdges: []dag.EdgeModification{
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

	err := rt.ModifyGraph(ctx, "nonexistent", dag.GraphModifications{})
	if err == nil {
		t.Error("Expected error for non-existent workflow")
	}
}

func TestModifyGraph_RollbackOnEdgeError(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	workflow, _ := helperSetupModifiableWorkflow(t, rt, "rollback-edge")

	graph := workflow.GetGraph()

	// 添加到不存在目标的边应该失败
	mods := dag.GraphModifications{
		AddEdges: []dag.EdgeModification{
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
	workflow, _ := helperSetupModifiableWorkflow(t, rt, "complex-mod")

	mods := dag.GraphModifications{
		RemoveEdges: []dag.EdgeRemoval{
			{Source: "B", Target: "C"},
		},
		AddNodes: []core.NodeConfig{
			{Name: "D", Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo D"}}},
		},
		AddEdges: []dag.EdgeModification{
			{Source: "B", Target: "D"},
		},
	}

	err := rt.ModifyGraph(ctx, "complex-mod", mods)
	if err != nil {
		t.Fatalf("ModifyGraph() error = %v", err)
	}

	graph := workflow.GetGraph()
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

// 节点替换（如续跑时仅修改 executor）会连带删除关联边；Graph 未变时
// UpdateConfig 不会重加边。替换后必须恢复边，否则节点变孤立根节点、
// 调度顺序丢失（曾导致续跑追加的链式节点被并行执行）
func TestModifyGraph_ReplaceNodePreservesEdges(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	workflow, _ := helperSetupModifiableWorkflow(t, rt, "replace-node")

	// B 未执行（StatusUnknown），替换其 executor；图结构不变
	mods := dag.GraphModifications{
		RemoveNodes: []string{"B"},
		AddNodes: []core.NodeConfig{
			{Id: "B", Name: "B", Executor: "local2", Steps: []core.Step{{Name: "step1", Run: "echo B"}}},
		},
	}

	if err := rt.ModifyGraph(ctx, "replace-node", mods); err != nil {
		t.Fatalf("ModifyGraph() error = %v", err)
	}

	graph := workflow.GetGraph()
	nodeB, ok := graph.GetNode("B")
	if !ok {
		t.Fatal("Node B should exist after replacement")
	}
	if nodeB.GetExecutor() != "local2" {
		t.Errorf("Node B executor = %v, want local2", nodeB.GetExecutor())
	}
	if _, ok := graph.GetEdge("A", "B"); !ok {
		t.Error("Edge A->B should be preserved after node replacement")
	}
	if _, ok := graph.GetEdge("B", "C"); !ok {
		t.Error("Edge B->C should be preserved after node replacement")
	}
}

// UpdateConfig 新增执行器条目时，必须同时持久化到存储配置并注册进运行中
// workflow 的 provider——否则续跑追加的异构节点（如 docker）运行时报
// executor config not found（曾导致续跑首轮失败）
func TestUpdateConfig_AddExecutorUsable(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	workflow, _ := helperSetupModifiableWorkflow(t, rt, "add-exec")

	newYAML := `Version: "1.0"
Name: modify-test
Executors:
  local:
    type: local
    config: {}
  local2:
    type: local
    config: {}
Graph: |
  stateDiagram-v2
    [*] --> A
    A --> B
    B --> C
    C --> D
    D --> [*]
Nodes:
  A:
    name: A
    executor: local
    steps:
      - name: step1
        run: echo A
  B:
    name: B
    executor: local
    steps:
      - name: step1
        run: echo B
  C:
    name: C
    executor: local
    steps:
      - name: step1
        run: echo C
  D:
    name: D
    executor: local2
    steps:
      - name: step1
        run: echo D
`
	if err := rt.UpdateConfig(ctx, "add-exec", newYAML); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	// 存储配置持久化新执行器
	cfg := rt.workflowConfigs["add-exec"]
	if _, ok := cfg.Executors["local2"]; !ok {
		t.Error("stored config should persist new executor local2")
	}

	// provider 可解析新执行器
	prov := workflow.(*dag.WorkflowImpl).GetExecutorProvider()
	if prov == nil {
		t.Fatal("executor provider should not be nil")
	}
	if _, err := prov.GetExecutor(ctx, "local2"); err != nil {
		t.Errorf("provider should resolve new executor local2: %v", err)
	}
	// 旧执行器仍可用
	if _, err := prov.GetExecutor(ctx, "local"); err != nil {
		t.Errorf("provider should still resolve local: %v", err)
	}

	// 新节点及其入边存在
	graph := workflow.GetGraph()
	if _, ok := graph.GetNode("D"); !ok {
		t.Error("Node D should exist")
	}
	if _, ok := graph.GetEdge("C", "D"); !ok {
		t.Error("Edge C->D should exist")
	}
}
