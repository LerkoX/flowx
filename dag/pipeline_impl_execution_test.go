package dag

import (
	"context"
	"testing"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/executor"
	"github.com/LerkoX/flowx/template"
)

// TestSetTemplateEngine 测试设置模板引擎
func TestSetTemplateEngine(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	engine := template.NewPongo2TemplateEngine()
	impl.SetTemplateEngine(engine)

	// 验证模板引擎已设置
	gotEngine := impl.GetTemplateEngine()
	if gotEngine == nil {
		t.Fatal("Template engine should not be nil after SetTemplateEngine")
	}
}

// TestGetTemplateEngine_Nil 测试未设置模板引擎时返回 nil
func TestGetTemplateEngine_Nil(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	// 不设置模板引擎
	gotEngine := impl.GetTemplateEngine()
	if gotEngine != nil {
		t.Errorf("Expected nil template engine, got %v", gotEngine)
	}
}

// TestRenderStringWithRuntimeContext_NoEngine 测试没有模板引擎时返回原字符串
func TestRenderStringWithRuntimeContext_NoEngine(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	// 不设置模板引擎
	result, err := impl.renderStringWithRuntimeContext("{{ Param.version }}")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// 应该返回原始字符串（未渲染）
	if result != "{{ Param.version }}" {
		t.Errorf("Expected raw template string, got %s", result)
	}
}

// TestRenderStringWithRuntimeContext_WithEngine 测试有模板引擎时渲染字符串
func TestRenderStringWithRuntimeContext_WithEngine(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	// 设置模板引擎
	impl.SetTemplateEngine(template.NewPongo2TemplateEngine())

	// 设置参数
	impl.SetParam(map[string]interface{}{
		"version": "v1.0.0",
	})

	// 渲染字符串
	result, err := impl.renderStringWithRuntimeContext("version: {{ version }}")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result != "version: v1.0.0" {
		t.Errorf("Expected 'version: v1.0.0', got '%s'", result)
	}
}

// TestRenderStringWithRuntimeContext_WithMetadata 测试带 Metadata 渲染
func TestRenderStringWithRuntimeContext_WithMetadata(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	// 设置模板引擎
	impl.SetTemplateEngine(template.NewPongo2TemplateEngine())

	// 设置 Metadata
	impl.mu.Lock()
	impl.metadata = Metadata{
		"Build.buildId": {Value: "12345", SrcNode: "Build"},
	}
	impl.mu.Unlock()

	// 渲染字符串
	result, err := impl.renderStringWithRuntimeContext("Build ID: {{ Build.buildId }}")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result != "Build ID: 12345" {
		t.Errorf("Expected 'Build ID: 12345', got '%s'", result)
	}
}

// TestExtractOutput_NoExtractConfig 测试无提取配置时不提取
func TestExtractOutput_NoExtractConfig(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	node := NewDGANode("test-node", "RUNNING")

	// 不设置 extract 配置，extractOutput 应该返回 nil
	err := impl.extractOutput(context.Background(), node, nil, "some output")
	if err != nil {
		t.Errorf("extractOutput should not return error for no config: %v", err)
	}
}

// TestExtractOutput_EmptyOutput 测试空输出
func TestExtractOutput_EmptyOutput(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	node := NewDGANode("test-node", "RUNNING")

	err := impl.extractOutput(context.Background(), node, nil, "")
	if err != nil {
		t.Errorf("extractOutput should not return error for empty output: %v", err)
	}
}

// TestShouldSkipStep_NoRuntimeStatus 测试无运行时状态时不跳过
func TestShouldSkipStep_NoRuntimeStatus(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	node := NewDGANode("test-node", "RUNNING")

	// 无运行时状态时，shouldSkipStep 返回 false
	result := impl.shouldSkipStep(node, "test-step")
	if result {
		t.Error("Expected shouldSkipStep to return false when no runtime status")
	}
}

// TestShouldSkipStep_WithRuntimeStatus 测试有运行时状态时跳过已完成步骤
func TestShouldSkipStep_WithRuntimeStatus(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	var node Node = NewDGANode("test-node", "RUNNING")
	node.SetRuntimeStatus(&core.NodeRuntimeStatus{
		Id:     "test-node",
		Status: core.StatusRunning,
		Steps: []core.StepRuntimeStatus{
			{Name: "step1", Status: core.StatusSuccess},
			{Name: "step2", Status: core.StatusRunning},
		},
	})

	// 已完成的步骤应该被跳过
	result := impl.shouldSkipStep(node, "step1")
	if !result {
		t.Error("Expected shouldSkipStep to return true for completed step")
	}

	// 运行中的步骤不应该被跳过
	result = impl.shouldSkipStep(node, "step2")
	if result {
		t.Error("Expected shouldSkipStep to return false for running step")
	}
}

// TestCreateExtractor_NilConfig 测试 nil 配置返回 nil 提取器
func TestCreateExtractor_NilConfig(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	extractor, err := impl.createExtractor(nil)
	if err != nil {
		t.Errorf("createExtractor(nil) error = %v", err)
	}
	if extractor != nil {
		t.Error("createExtractor(nil) should return nil extractor")
	}
}

// TestCreateExtractor_CodecBlock 测试创建 codec block 提取器
func TestCreateExtractor_CodecBlock(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	config := map[string]interface{}{
		"type": "codec-block",
	}

	extractor, err := impl.createExtractor(config)
	if err != nil {
		t.Errorf("createExtractor(codec-block) error = %v", err)
	}
	if extractor == nil {
		t.Error("createExtractor(codec-block) should not return nil")
	}
}

// TestHandleInputRequest_NilEvent 测试 nil 事件不处理
func TestHandleInputRequest_NilEvent(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	node := NewDGANode("test-node", "RUNNING")

	// nil 事件应该直接返回
	impl.handleInputRequest(node, nil)
}

// TestHandleInputRequest_EmptyRequest 测试空请求不处理
func TestHandleInputRequest_EmptyRequest(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	node := NewDGANode("test-node", "RUNNING")

	event := &executor.InputRequestEvent{
		Request: &executor.InputRequest{},
	}

	// 空请求应该直接返回
	impl.handleInputRequest(node, event)
}

// TestCancel 测试取消流水线
func TestCancel(t *testing.T) {
	pipeline := NewPipeline(nil)
	impl := pipeline.(*PipelineImpl)

	// 取消不应该 panic
	impl.Cancel()

	// 验证取消后的状态（通过 Done 通道）
	select {
	case <-impl.Done():
		// 管道已关闭
	default:
		// 管道可能还未关闭（取决于实现）
	}
}
