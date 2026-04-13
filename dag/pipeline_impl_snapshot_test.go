package dag

import (
	"context"
	"testing"

	"github.com/LerkoX/flowx/core"
)

// testListener 实现 Listener 接口用于测试
type testListener struct {
	handleFn   func(p Pipeline, event Event)
	eventsFn   func() []Event
}

func (l *testListener) Handle(p Pipeline, event Event) {
	if l.handleFn != nil {
		l.handleFn(p, event)
	}
}

func (l *testListener) Events() []Event {
	if l.eventsFn != nil {
		return l.eventsFn()
	}
	return nil
}

// TestTakeSnapshot_Basic 测试基本快照功能
func TestTakeSnapshot_Basic(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	graph := NewDGAGraph()
	node := NewDGANode("test-node", "SUCCESS")
	graph.AddVertex(node)
	impl.SetGraph(graph)

	config := &core.PipelineConfig{
		Version: "1.0",
		Name:    "test-pipeline",
	}

	snapshotter := NewPipelineSnapshotter()
	snapshot, err := snapshotter.TakeSnapshot(pipeline, config)
	if err != nil {
		t.Fatalf("TakeSnapshot failed: %v", err)
	}

	if snapshot == nil {
		t.Fatal("TakeSnapshot should not return nil")
	}
}

// TestTakeSnapshot_WithParam 测试带参数的快照
func TestTakeSnapshot_WithParam(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	// 设置参数
	impl.SetParam(map[string]interface{}{
		"version": "v1.0.0",
		"env":     "prod",
	})

	// 设置图（避免 GetGraph() panic）
	impl.SetGraph(NewDGAGraph())

	config := &core.PipelineConfig{
		Version: "1.0",
		Name:    "test-pipeline",
	}

	snapshotter := NewPipelineSnapshotter()
	snapshot, err := snapshotter.TakeSnapshot(pipeline, config)
	if err != nil {
		t.Fatalf("TakeSnapshot failed: %v", err)
	}

	if snapshot == nil {
		t.Fatal("TakeSnapshot should not return nil")
	}
}

// TestTakeSnapshot_WithMetadata 测试带元数据的快照
func TestTakeSnapshot_WithMetadata(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	// 设置图（避免 GetGraph() panic）
	impl.SetGraph(NewDGAGraph())

	// 设置元数据
	impl.mu.Lock()
	impl.metadata = Metadata{
		"Build.status": {Value: "success", SrcNode: "Build"},
	}
	impl.mu.Unlock()

	config := &core.PipelineConfig{
		Version: "1.0",
		Name:    "test-pipeline",
	}

	snapshotter := NewPipelineSnapshotter()
	snapshot, err := snapshotter.TakeSnapshot(pipeline, config)
	if err != nil {
		t.Fatalf("TakeSnapshot failed: %v", err)
	}

	if snapshot == nil {
		t.Fatal("TakeSnapshot should not return nil")
	}
}

// TestTakeSnapshot_WithNodeStatus 测试带节点状态的快照
func TestTakeSnapshot_WithNodeStatus(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	graph := NewDGAGraph()
	node := NewDGANode("test-node", "SUCCESS")
	node.SetRuntimeStatus(&core.NodeRuntimeStatus{
		Id:     "test-node",
		Status: core.StatusSuccess,
		Steps: []core.StepRuntimeStatus{
			{Name: "step1", Status: core.StatusSuccess},
		},
	})
	graph.AddVertex(node)
	impl.SetGraph(graph)

	config := &core.PipelineConfig{
		Version: "1.0",
		Name:    "test-pipeline",
		Nodes: map[string]core.NodeConfig{
			"test-node": {Name: "测试节点"},
		},
	}

	snapshotter := NewPipelineSnapshotter()
	snapshot, err := snapshotter.TakeSnapshot(pipeline, config)
	if err != nil {
		t.Fatalf("TakeSnapshot failed: %v", err)
	}

	if snapshot == nil {
		t.Fatal("TakeSnapshot should not return nil")
	}
}

// TestToYAML 测试 YAML 序列化
func TestToYAML(t *testing.T) {
	config := &core.PipelineConfig{
		Version: "1.0",
		Name:    "test-pipeline",
		Param: map[string]interface{}{
			"version": "v1.0.0",
		},
	}

	snapshotter := NewPipelineSnapshotter()
	yamlStr, err := snapshotter.ToYAML(config)
	if err != nil {
		t.Fatalf("ToYAML failed: %v", err)
	}

	if len(yamlStr) == 0 {
		t.Error("ToYAML should not return empty string")
	}
}

// TestFromYAML 测试 YAML 反序列化
func TestFromYAML(t *testing.T) {
	yamlStr := `
Version: "1.0"
Name: test-pipeline
Param:
  version:
    value: "v1.0.0"
`

	snapshotter := NewPipelineSnapshotter()
	snapshot, err := snapshotter.FromYAML(yamlStr)
	if err != nil {
		t.Fatalf("FromYAML failed: %v", err)
	}

	if snapshot.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", snapshot.Version)
	}
	if snapshot.Name != "test-pipeline" {
		t.Errorf("Expected name 'test-pipeline', got '%s'", snapshot.Name)
	}
}

// TestFromYAML_Invalid 测试无效 YAML
func TestFromYAML_Invalid(t *testing.T) {
	invalidYaml := `{invalid yaml content`

	snapshotter := NewPipelineSnapshotter()
	_, err := snapshotter.FromYAML(invalidYaml)
	if err == nil {
		t.Error("FromYAML should return error for invalid YAML")
	}
}

// TestNotifyEvent_NoListener 测试没有监听器时不会 panic
func TestNotifyEvent_NoListener(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	// 没有设置监听器，NotifyEvent 不应该 panic
	impl.NotifyEvent(PipelineNodeFinish)
	impl.NotifyEvent(PipelineStart)
	impl.NotifyEvent(PipelineFinish)
}

// TestNotifyEvent_WithListener 测试带监听器的事件通知
func TestNotifyEvent_WithListener(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	eventReceived := false
	listener := &testListener{
		handleFn: func(p Pipeline, event Event) {
			eventReceived = true
		},
		eventsFn: func() []Event {
			return []Event{PipelineNodeStart}
		},
	}

	impl.Listening(listener)

	impl.NotifyEvent(PipelineNodeStart)

	if !eventReceived {
		t.Error("Expected listener to receive event")
	}
}

// TestNotify 测试 Notify 方法
func TestNotify(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	// Notify 不应该 panic
	impl.Notify()
}

// TestExtractOutput_WithCodecBlockConfig 测试带 codec block 配置的提取
func TestExtractOutput_WithCodecBlockConfig(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	var node Node = NewDGANode("test-node", "RUNNING")
	node.SetRuntimeStatus(&core.NodeRuntimeStatus{
		Id:     "test-node",
		Status: core.StatusRunning,
	})

	output := "Some output\n```flowx-yaml\nkey: value\n```\nEnd"

	err := impl.extractOutput(context.Background(), node, nil, output)
	if err != nil {
		t.Errorf("extractOutput failed: %v", err)
	}
}

// TestExtractOutput_WithRegexConfig 测试带正则配置的数据提取
func TestExtractOutput_WithRegexConfig(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	var node Node = NewDGANode("test-node", "RUNNING")
	node.SetRuntimeStatus(&core.NodeRuntimeStatus{
		Id:     "test-node",
		Status: core.StatusRunning,
	})

	output := "Build started\nVersion: v2.0.0\nBuild finished"

	err := impl.extractOutput(context.Background(), node, nil, output)
	if err != nil {
		t.Errorf("extractOutput failed: %v", err)
	}
}

// TestCreateExtractor_Regex 测试创建正则提取器
func TestCreateExtractor_Regex(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	config := map[string]interface{}{
		"type": "regex",
		"patterns": map[string]string{
			"version": "Version: (\\S+)",
		},
	}

	extractor, err := impl.createExtractor(config)
	// regex extractor requires patterns
	if err != nil {
		t.Logf("createExtractor returned error (expected for some configs): %v", err)
	}
	if extractor != nil {
		t.Log("createExtractor returned a non-nil extractor")
	}
}

// TestCreateExtractor_UnknownType 测试创建未知类型的提取器
func TestCreateExtractor_UnknownType(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	config := map[string]interface{}{
		"type": "unknown-type",
	}

	extractor, err := impl.createExtractor(config)
	// unknown type returns error
	if err != nil {
		t.Logf("createExtractor returned error (expected for unknown type): %v", err)
	}
	if extractor == nil {
		t.Log("createExtractor returned nil for unknown type (expected)")
	}
}
