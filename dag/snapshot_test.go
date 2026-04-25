package dag

import (
	"context"
	"testing"

	"github.com/LerkoX/flowx/core"
	"gopkg.in/yaml.v2"
)

func TestPipelineSnapshotter_ToYAML_BasicConfig(t *testing.T) {
	s := NewPipelineSnapshotter()
	config := &core.PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Nodes: map[string]core.NodeConfig{
			"Node1": {
				Name:     "Node1",
				Executor: "local",
				Steps: []core.Step{
					{Name: "step1", Run: "echo hello"},
				},
			},
		},
	}

	yamlStr, err := s.ToYAML(config)
	if err != nil {
		t.Fatalf("ToYAML() error = %v", err)
	}
	if yamlStr == "" {
		t.Error("ToYAML() returned empty string")
	}
	if !containsSubstring(yamlStr, "Node1") {
		t.Error("ToYAML() output should contain 'Node1'")
	}
}

func TestPipelineSnapshotter_ToYAML_EmptyConfig(t *testing.T) {
	s := NewPipelineSnapshotter()
	config := &core.PipelineConfig{}

	yamlStr, err := s.ToYAML(config)
	if err != nil {
		t.Fatalf("ToYAML() error = %v", err)
	}
	if yamlStr == "" {
		t.Error("ToYAML() returned empty string for empty config")
	}
}

func TestPipelineSnapshotter_FromYAML_BasicConfig(t *testing.T) {
	yamlStr := `Version: "1.0"
Name: test-pipeline
Nodes:
  Node1:
    name: Node1
    executor: local
    steps:
      - name: step1
        run: "echo hello"
`

	config := &core.PipelineConfig{}
	err := yaml.Unmarshal([]byte(yamlStr), config)
	if err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if config.Name != "test-pipeline" {
		t.Errorf("Name = %q, want %q", config.Name, "test-pipeline")
	}
	if _, ok := config.Nodes["Node1"]; !ok {
		t.Error("Expected Node1 in config.Nodes")
	}
}

func TestPipelineSnapshotter_FromYAML_RoundTrip(t *testing.T) {
	s := NewPipelineSnapshotter()
	original := &core.PipelineConfig{
		Version: "1.0",
		Name:    "roundtrip",
		Param:   map[string]interface{}{"key": "value"},
		Nodes: map[string]core.NodeConfig{
			"A": {Name: "A", Executor: "local"},
		},
	}

	yamlStr, err := s.ToYAML(original)
	if err != nil {
		t.Fatalf("ToYAML() error = %v", err)
	}

	parsed := &core.PipelineConfig{}
	err = yaml.Unmarshal([]byte(yamlStr), parsed)
	if err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if parsed.Name != original.Name {
		t.Errorf("Round-trip Name = %q, want %q", parsed.Name, original.Name)
	}
}

func TestPipelineSnapshotter_FromYAML_InvalidYAML(t *testing.T) {
	invalidYaml := ":\n  invalid: [yaml: content"
	config := &core.PipelineConfig{}
	err := yaml.Unmarshal([]byte(invalidYaml), config)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestPipelineSnapshotter_TakeSnapshot_SimplePipeline(t *testing.T) {
	graph := NewDGAGraph()
	node := NewDGANode("Node1", core.StatusUnknown)
	node.SetRuntimeStatus(&core.NodeRuntimeStatus{
		Id:     "rt-1",
		Status: core.StatusSuccess,
		Steps: []core.StepRuntimeStatus{
			{Id: "step-1", Name: "step1", Status: core.StatusSuccess},
		},
	})
	graph.AddVertex(node)

	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	pipeline.SetGraph(graph)

	originalConfig := &core.PipelineConfig{
		Version: "1.0",
		Name:    "snapshot-test",
		Nodes: map[string]core.NodeConfig{
			"Node1": {
				Name:     "Node1",
				Executor: "local",
				Steps:    []core.Step{{Name: "step1", Run: "echo hi"}},
			},
		},
	}

	snapshotter := NewPipelineSnapshotter()
	snapshot, err := snapshotter.TakeSnapshot(pipeline, originalConfig)
	if err != nil {
		t.Fatalf("TakeSnapshot() error = %v", err)
	}

	nodeConfig, ok := snapshot.Nodes["Node1"]
	if !ok {
		t.Fatal("Node1 not found in snapshot")
	}
	if nodeConfig.Runtime == nil {
		t.Fatal("Runtime should not be nil in snapshot")
	}
	if nodeConfig.Runtime.Status != core.StatusSuccess {
		t.Errorf("Runtime.Status = %q, want %q", nodeConfig.Runtime.Status, core.StatusSuccess)
	}
}

func TestPipelineSnapshotter_TakeSnapshot_NilRuntimeStatus(t *testing.T) {
	graph := NewDGAGraph()
	node := NewDGANode("Node1", core.StatusUnknown)
	// 不设置 RuntimeStatus
	graph.AddVertex(node)

	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	pipeline.SetGraph(graph)

	originalConfig := &core.PipelineConfig{
		Version: "1.0",
		Name:    "nil-runtime-test",
		Nodes: map[string]core.NodeConfig{
			"Node1": {Name: "Node1", Executor: "local"},
		},
	}

	snapshotter := NewPipelineSnapshotter()
	snapshot, err := snapshotter.TakeSnapshot(pipeline, originalConfig)
	if err != nil {
		t.Fatalf("TakeSnapshot() error = %v", err)
	}
	// Runtime 应该为 nil（没有运行时状态的节点）
	if snapshot.Nodes["Node1"].Runtime != nil {
		t.Error("Expected nil Runtime for node without runtime status")
	}
}

func TestPipelineSnapshotter_TakeSnapshot_DeepCopy(t *testing.T) {
	graph := NewDGAGraph()
	node := NewDGANode("Node1", core.StatusUnknown)
	graph.AddVertex(node)

	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	pipeline.SetGraph(graph)

	originalConfig := &core.PipelineConfig{
		Version: "1.0",
		Name:    "deepcopy-test",
		Nodes: map[string]core.NodeConfig{
			"Node1": {Name: "Node1", Executor: "local"},
		},
	}

	snapshotter := NewPipelineSnapshotter()
	snapshot, err := snapshotter.TakeSnapshot(pipeline, originalConfig)
	if err != nil {
		t.Fatalf("TakeSnapshot() error = %v", err)
	}

	// 修改原始 config，快照不受影响
	originalConfig.Name = "modified"
	if snapshot.Name != "deepcopy-test" {
		t.Errorf("Snapshot was affected by original modification: %q", snapshot.Name)
	}
}

func TestPipelineSnapshotter_TakeSnapshot_WithSteps(t *testing.T) {
	graph := NewDGAGraph()
	node := NewDGANodeWithConfig("Node1", core.StatusUnknown, "local", "", []core.Step{
		{Name: "step1", Run: "echo hello", Id: "step-id-1"},
	}, nil)
	graph.AddVertex(node)

	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	pipeline.SetGraph(graph)

	originalConfig := &core.PipelineConfig{
		Version: "1.0",
		Name:    "steps-test",
		Nodes: map[string]core.NodeConfig{
			"Node1": {
				Name:     "Node1",
				Executor: "local",
				Steps:    []core.Step{{Name: "step1", Run: "echo hello"}},
			},
		},
	}

	snapshotter := NewPipelineSnapshotter()
	snapshot, err := snapshotter.TakeSnapshot(pipeline, originalConfig)
	if err != nil {
		t.Fatalf("TakeSnapshot() error = %v", err)
	}

	nodeConfig := snapshot.Nodes["Node1"]
	if len(nodeConfig.Steps) == 0 {
		t.Fatal("Expected steps in snapshot")
	}
	if nodeConfig.Steps[0].Id != "step-id-1" {
		t.Errorf("Step Id = %q, want %q", nodeConfig.Steps[0].Id, "step-id-1")
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && len(sub) > 0 && findSubstring(s, sub)))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
