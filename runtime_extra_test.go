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

// TestRuntimeImpl_UpdateConfig_AddNodesWithDisplayName 回归测试（studio 执行 96 事故）：
// nodeRef 展开会给节点填显示名（如 echo 包的「回声」），Name 与 map key 不同。
// 追加多个这样的节点时，必须以 map key 作为节点 ID，不能用 Name——
// 否则所有同名显示名的节点会坍缩覆盖为同一个节点
func TestRuntimeImpl_UpdateConfig_AddNodesWithDisplayName(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)

	graph := dag.NewDGAGraph()
	nodeA := dag.NewDGANodeWithConfig("A", core.StatusSuccess, "local", "", []core.Step{{Name: "step1", Run: "echo A"}}, nil)
	nodeA.SetRuntimeStatus(&core.NodeRuntimeStatus{Status: core.StatusSuccess})
	graph.AddVertex(nodeA)

	pipeline := dag.NewPipeline(ctx).(*dag.PipelineImpl)
	pipeline.SetGraph(graph)
	pipeline.SetStatusForTest(core.StatusSuccess)

	execProvider := provider.NewProvider()
	execProvider.RegisterExecutor("local", provider.ExecutorConfig{Type: "local", Config: map[string]interface{}{}})
	pipeline.SetExecutorProvider(execProvider)

	rt.pipelines["update-display-name"] = pipeline
	rt.pipelineConfigs["update-display-name"] = &core.PipelineConfig{
		Version: "1.0",
		Name:    "update-display-name",
		Executors: map[string]core.ExecutorConfig{
			"local": {Type: "local", Config: map[string]interface{}{}},
		},
		Graph: "stateDiagram-v2\n  [*] --> A\n  A --> [*]",
		Nodes: map[string]core.NodeConfig{
			"A": {Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo A"}}},
		},
	}

	// 新配置：追加 3 个带显示名「回声」（与 map key 不同）的节点
	newConfig := `
Version: "1.0"
Name: update-display-name

Executors:
  local:
    type: local
    config: {}

Graph: |
  stateDiagram-v2
    [*] --> A
    A --> tail5
    tail5 --> tail6
    tail6 --> tail7
    tail7 --> [*]

Nodes:
  A:
    executor: local
    steps:
      - name: step1
        run: echo A
  tail5:
    name: 回声
    executor: local
    steps:
      - name: step1
        run: echo 5
  tail6:
    name: 回声
    executor: local
    steps:
      - name: step1
        run: echo 6
  tail7:
    name: 回声
    executor: local
    steps:
      - name: step1
        run: echo 7
`

	if err := rt.UpdateConfig(ctx, "update-display-name", newConfig); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	g := pipeline.GetGraph()
	// 三个节点应以各自 map key 为 ID 独立存在，而不是坍缩为「回声」
	for _, id := range []string{"tail5", "tail6", "tail7"} {
		if _, ok := g.GetNode(id); !ok {
			t.Errorf("Node %s should exist in graph", id)
		}
	}
	if _, ok := g.GetNode("回声"); ok {
		t.Error("Node 回声 should not exist (display name must not become node ID)")
	}

	// 存储配置的 Nodes 键也应为 map key
	cfg := rt.pipelineConfigs["update-display-name"]
	for _, id := range []string{"tail5", "tail6", "tail7"} {
		if _, ok := cfg.Nodes[id]; !ok {
			t.Errorf("Stored config should contain node key %s", id)
		}
	}
}

// TestRuntimeImpl_UpdateConfig_AddExecutorAllowed 测试续跑追加节点时允许新增执行器条目，
// 但已有条目不可修改/删除
func TestRuntimeImpl_UpdateConfig_AddExecutorAllowed(t *testing.T) {
	ctx := context.Background()

	setup := func(id string) *RuntimeImpl {
		rt := NewRuntime(ctx).(*RuntimeImpl)
		graph := dag.NewDGAGraph()
		nodeA := dag.NewDGANodeWithConfig("A", core.StatusSuccess, "local", "", []core.Step{{Name: "step1", Run: "echo A"}}, nil)
		nodeA.SetRuntimeStatus(&core.NodeRuntimeStatus{Status: core.StatusSuccess})
		graph.AddVertex(nodeA)

		pipeline := dag.NewPipeline(ctx).(*dag.PipelineImpl)
		pipeline.SetGraph(graph)
		pipeline.SetStatusForTest(core.StatusSuccess)

		execProvider := provider.NewProvider()
		execProvider.RegisterExecutor("local", provider.ExecutorConfig{Type: "local", Config: map[string]interface{}{}})
		execProvider.RegisterExecutor("remote-docker", provider.ExecutorConfig{Type: "docker", Config: map[string]interface{}{"host": "ssh://remote"}})
		pipeline.SetExecutorProvider(execProvider)

		rt.pipelines[id] = pipeline
		rt.pipelineConfigs[id] = &core.PipelineConfig{
			Version: "1.0",
			Name:    "exec-add-test",
			Executors: map[string]core.ExecutorConfig{
				"local": {Type: "local", Config: map[string]interface{}{}},
			},
			Graph: "stateDiagram-v2\n  [*] --> A\n  A --> [*]",
			Nodes: map[string]core.NodeConfig{
				"A": {Executor: "local", Steps: []core.Step{{Name: "step1", Run: "echo A"}}},
			},
		}
		return rt
	}

	// 1. 新增执行器条目 + 新节点引用它：允许
	t.Run("add new executor entry", func(t *testing.T) {
		rt := setup("exec-add-allow")
		newConfig := `
Version: "1.0"
Name: exec-add-test

Executors:
  local:
    type: local
    config: {}
  remote-docker:
    type: docker
    config:
      host: ssh://remote

Graph: |
  stateDiagram-v2
    [*] --> A
    A --> B
    B --> [*]

Nodes:
  A:
    executor: local
    steps:
      - name: step1
        run: echo A
  B:
    executor: remote-docker
    steps:
      - name: step1
        run: echo B
`
		if err := rt.UpdateConfig(ctx, "exec-add-allow", newConfig); err != nil {
			t.Fatalf("UpdateConfig should allow adding new executor entry, got: %v", err)
		}
	})

	// 2. 修改已有执行器条目：拒绝
	t.Run("modify existing executor", func(t *testing.T) {
		rt := setup("exec-add-modify")
		newConfig := `
Version: "1.0"
Name: exec-add-test

Executors:
  local:
    type: local
    config:
      shell: zsh

Graph: |
  stateDiagram-v2
    [*] --> A
    A --> [*]

Nodes:
  A:
    executor: local
    steps:
      - name: step1
        run: echo A
`
		err := rt.UpdateConfig(ctx, "exec-add-modify", newConfig)
		if err == nil {
			t.Fatal("expected error when modifying existing executor")
		}
		t.Logf("got expected error: %v", err)
	})

	// 3. 删除已有执行器条目：拒绝
	t.Run("remove existing executor", func(t *testing.T) {
		rt := setup("exec-add-remove")
		newConfig := `
Version: "1.0"
Name: exec-add-test

Executors:
  remote-docker:
    type: docker
    config:
      host: ssh://remote

Graph: |
  stateDiagram-v2
    [*] --> A
    A --> [*]

Nodes:
  A:
    executor: remote-docker
    steps:
      - name: step1
        run: echo A
`
		err := rt.UpdateConfig(ctx, "exec-add-remove", newConfig)
		if err == nil {
			t.Fatal("expected error when removing existing executor")
		}
		t.Logf("got expected error: %v", err)
	})
}
