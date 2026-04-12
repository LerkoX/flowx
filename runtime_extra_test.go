package flowx

import (
	"context"
	"testing"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/dag"
	"github.com/LerkoX/flowx/executor/provider"
)

// --- UpdateConfig Tests ---

// TestRuntimeImpl_UpdateConfig_AddNodes 测试通过新配置添加节点
func TestRuntimeImpl_UpdateConfig_AddNodes(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)

	// 手动构建 pipeline（A 已执行，B 未执行）
	graph := dag.NewDGAGraph()
	nodeA := dag.NewDGANodeWithConfig("A", core.StatusSuccess, "local", "", []core.Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&core.NodeRuntimeStatus{Status: core.StatusSuccess})
	nodeB := dag.NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo B"}}, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddEdge(dag.NewDGAEdge(nodeA, nodeB))

	pipeline := dag.NewPipeline(ctx).(*dag.PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetStatusForTest(core.StatusSuccess)

	execProvider := provider.NewProvider()
	execProvider.RegisterExecutor("local", provider.ExecutorConfig{Type: "local", Config: map[string]interface{}{}})
	pipeline.SetExecutorProvider(execProvider)

	rt.pipelines["update-add-test"] = pipeline
	rt.pipelineConfigs["update-add-test"] = &core.PipelineConfig{
		Version: "1.0",
		Name:    "update-test",
		Executors: map[string]core.ExecutorConfig{
			"local": {Type: "local", Config: map[string]interface{}{}},
		},
		Graph: "stateDiagram-v2\n  [*] --> A\n  A --> B\n  B --> [*]",
		Nodes: map[string]core.NodeConfig{
			"A": {Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo A"}}},
			"B": {Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo B"}}},
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
	graph := dag.NewDGAGraph()
	nodeA := dag.NewDGANodeWithConfig("A", core.StatusSuccess, "local", "", []core.Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&core.NodeRuntimeStatus{Status: core.StatusSuccess})
	nodeB := dag.NewDGANodeWithConfig("B", core.StatusUnknown, "local", "", []core.Step{{Name: "step1", Run: "echo B"}}, nil)

	graph.AddVertex(nodeA)
	graph.AddVertex(nodeB)
	graph.AddEdge(dag.NewDGAEdge(nodeA, nodeB))

	pipeline := dag.NewPipeline(ctx).(*dag.PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetStatusForTest(core.StatusSuccess)

	execProvider := provider.NewProvider()
	execProvider.RegisterExecutor("local", provider.ExecutorConfig{Type: "local", Config: map[string]interface{}{}})
	pipeline.SetExecutorProvider(execProvider)

	rt.pipelines["remove-unexec"] = pipeline
	rt.pipelineConfigs["remove-unexec"] = &core.PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Executors: map[string]core.ExecutorConfig{
			"local": {Type: "local", Config: map[string]interface{}{}},
		},
		Nodes: map[string]core.NodeConfig{
			"A": {Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo A"}}},
			"B": {Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo B"}}},
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

	graph := dag.NewDGAGraph()
	nodeA := dag.NewDGANodeWithConfig("A", core.StatusSuccess, "local", "", []core.Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&core.NodeRuntimeStatus{Status: core.StatusSuccess})

	graph.AddVertex(nodeA)

	pipeline := dag.NewPipeline(ctx).(*dag.PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetStatusForTest(core.StatusSuccess)

	rt.pipelines["remove-exec"] = pipeline
	rt.pipelineConfigs["remove-exec"] = &core.PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Executors: map[string]core.ExecutorConfig{
			"local": {Type: "local", Config: map[string]interface{}{}},
		},
		Nodes: map[string]core.NodeConfig{
			"A": {Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo A"}}},
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

	graph := dag.NewDGAGraph()
	nodeA := dag.NewDGANodeWithConfig("A", core.StatusSuccess, "local", "", []core.Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&core.NodeRuntimeStatus{Status: core.StatusSuccess})

	graph.AddVertex(nodeA)

	pipeline := dag.NewPipeline(ctx).(*dag.PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetStatusForTest(core.StatusSuccess)

	rt.pipelines["modify-exec"] = pipeline
	rt.pipelineConfigs["modify-exec"] = &core.PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Executors: map[string]core.ExecutorConfig{
			"local": {Type: "local", Config: map[string]interface{}{}},
		},
		Nodes: map[string]core.NodeConfig{
			"A": {Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo A"}}},
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

	graph := dag.NewDGAGraph()
	nodeA := dag.NewDGANodeWithConfig("A", core.StatusSuccess, "local", "", []core.Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&core.NodeRuntimeStatus{Status: core.StatusSuccess})

	graph.AddVertex(nodeA)

	pipeline := dag.NewPipeline(ctx).(*dag.PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetStatusForTest(core.StatusSuccess)

	rt.pipelines["immutable-test"] = pipeline
	rt.pipelineConfigs["immutable-test"] = &core.PipelineConfig{
		Version: "1.0",
		Name:    "original",
		Executors: map[string]core.ExecutorConfig{
			"local": {Type: "local", Config: map[string]interface{}{}},
		},
		Nodes: map[string]core.NodeConfig{
			"A": {Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo A"}}},
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

	graph := dag.NewDGAGraph()
	nodeA := dag.NewDGANodeWithConfig("A", core.StatusSuccess, "local", "", []core.Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&core.NodeRuntimeStatus{Status: core.StatusSuccess})

	graph.AddVertex(nodeA)

	pipeline := dag.NewPipeline(ctx).(*dag.PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetStatusForTest(core.StatusSuccess)

	rt.pipelines["nochange-test"] = pipeline
	rt.pipelineConfigs["nochange-test"] = &core.PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Executors: map[string]core.ExecutorConfig{
			"local": {Type: "local", Config: map[string]interface{}{}},
		},
		Nodes: map[string]core.NodeConfig{
			"A": {Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo A"}}},
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
