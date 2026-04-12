package pipelinex

import (
	"context"
	"strings"
	"testing"

	"github.com/LerkoX/pipelinex/logger"
)

func TestRuntimeImpl_SetPusher(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx).(*RuntimeImpl)

	pusher := logger.NewConsolePusher()
	runtime.SetPusher(pusher)

	runtime.mu.RLock()
	p := runtime.pusher
	runtime.mu.RUnlock()

	if p != pusher {
		t.Error("SetPusher did not store the pusher")
	}
}

func TestRuntimeImpl_SetTemplateEngine_Custom(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx).(*RuntimeImpl)

	engine := NewPongo2TemplateEngine()
	runtime.SetTemplateEngine(engine)

	got := runtime.GetTemplateEngine()
	if got == nil {
		t.Error("GetTemplateEngine() returned nil after SetTemplateEngine")
	}
}

func TestRuntimeImpl_SetTemplateEngine_Nil(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx).(*RuntimeImpl)

	runtime.SetTemplateEngine(nil)

	// getTemplateEngine 应返回默认 Pongo2 引擎
	engine := runtime.getTemplateEngine()
	if engine == nil {
		t.Error("getTemplateEngine() should return default engine when nil is set")
	}
}

func TestRuntimeImpl_Pause_NotFound(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx).(*RuntimeImpl)

	err := runtime.Pause(ctx, "nonexistent-id")
	if err == nil {
		t.Error("Expected error for non-existent pipeline")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should contain 'not found', got: %v", err)
	}
}

func TestRuntimeImpl_Resume_NotFound(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx).(*RuntimeImpl)

	err := runtime.Resume(ctx, "nonexistent-id")
	if err == nil {
		t.Error("Expected error for non-existent pipeline")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should contain 'not found', got: %v", err)
	}
}

func TestRuntimeImpl_CleanupCompletedPipelines(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx).(*RuntimeImpl)

	// 手动创建一个已完成的 pipeline 并注册到 runtime
	pipeline := NewPipeline(ctx).(*PipelineImpl)
	close(pipeline.doneChan) // 模拟已完成

	runtime.mu.Lock()
	runtime.pipelines["completed-pipeline"] = pipeline
	runtime.mu.Unlock()

	// 手动创建一个运行中的 pipeline
	runningPipeline := NewPipeline(ctx).(*PipelineImpl)
	runtime.mu.Lock()
	runtime.pipelines["running-pipeline"] = runningPipeline
	runtime.mu.Unlock()

	// 执行清理
	runtime.cleanupCompletedPipelines()

	// 验证已完成的 pipeline 被清理
	runtime.mu.RLock()
	_, completedExists := runtime.pipelines["completed-pipeline"]
	_, runningExists := runtime.pipelines["running-pipeline"]
	runtime.mu.RUnlock()

	if completedExists {
		t.Error("Completed pipeline should have been cleaned up")
	}
	if !runningExists {
		t.Error("Running pipeline should not have been cleaned up")
	}
}

func TestSetPipelineParam(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	param := map[string]interface{}{"key1": "value1", "key2": 42}

	SetPipelineParam(pipeline, param)

	if pipeline.param["key1"] != "value1" {
		t.Errorf("param[key1] = %v, want 'value1'", pipeline.param["key1"])
	}
	if pipeline.param["key2"] != 42 {
		t.Errorf("param[key2] = %v, want 42", pipeline.param["key2"])
	}
}

func TestSetPipelineParam_NonPipelineImpl(t *testing.T) {
	// 传入 nil 不会 panic
	SetPipelineParam(nil, map[string]interface{}{"key": "value"})
}

func TestValidateImmutableFields_NoChanges(t *testing.T) {
	old := &PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Param:   map[string]interface{}{"k": "v"},
	}
	err := validateImmutableFields(old, old)
	if err != nil {
		t.Errorf("Expected nil for identical configs, got: %v", err)
	}
}

func TestValidateImmutableFields_VersionChanged(t *testing.T) {
	old := &PipelineConfig{Version: "1.0", Name: "test"}
	newCfg := &PipelineConfig{Version: "2.0", Name: "test"}

	err := validateImmutableFields(old, newCfg)
	if err == nil {
		t.Error("Expected error when Version changed")
	}
	if !strings.Contains(err.Error(), "Version") {
		t.Errorf("Error should mention Version, got: %v", err)
	}
}

func TestValidateImmutableFields_NameChanged(t *testing.T) {
	old := &PipelineConfig{Version: "1.0", Name: "test"}
	newCfg := &PipelineConfig{Version: "1.0", Name: "changed"}

	err := validateImmutableFields(old, newCfg)
	if err == nil {
		t.Error("Expected error when Name changed")
	}
}

func TestValidateImmutableFields_MaxLoopIterationsChanged(t *testing.T) {
	old := &PipelineConfig{Version: "1.0", Name: "test", MaxLoopIterations: 100}
	newCfg := &PipelineConfig{Version: "1.0", Name: "test", MaxLoopIterations: 200}

	err := validateImmutableFields(old, newCfg)
	if err == nil {
		t.Error("Expected error when MaxLoopIterations changed")
	}
}

func TestValidateImmutableFields_ParamChanged(t *testing.T) {
	old := &PipelineConfig{Version: "1.0", Name: "test", Param: map[string]interface{}{"k": "v1"}}
	newCfg := &PipelineConfig{Version: "1.0", Name: "test", Param: map[string]interface{}{"k": "v2"}}

	err := validateImmutableFields(old, newCfg)
	if err == nil {
		t.Error("Expected error when Param changed")
	}
}

func TestValidateImmutableFields_ExecutorsChanged(t *testing.T) {
	old := &PipelineConfig{
		Version:   "1.0",
		Name:      "test",
		Executors: map[string]ExecutorConfig{"local": {Type: "local"}},
	}
	newCfg := &PipelineConfig{
		Version:   "1.0",
		Name:      "test",
		Executors: map[string]ExecutorConfig{"docker": {Type: "docker"}},
	}

	err := validateImmutableFields(old, newCfg)
	if err == nil {
		t.Error("Expected error when Executors changed")
	}
}

func TestValidateImmutableFields_LoggingChanged(t *testing.T) {
	old := &PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Logging: LoggingConfig{Endpoint: "http://old"},
	}
	newCfg := &PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Logging: LoggingConfig{Endpoint: "http://new"},
	}

	err := validateImmutableFields(old, newCfg)
	if err == nil {
		t.Error("Expected error when Logging changed")
	}
}

func TestValidateImmutableFields_AIChanged(t *testing.T) {
	old := &PipelineConfig{
		Version: "1.0",
		Name:    "test",
		AI:      AIConfig{Intent: "old"},
	}
	newCfg := &PipelineConfig{
		Version: "1.0",
		Name:    "test",
		AI:      AIConfig{Intent: "new"},
	}

	err := validateImmutableFields(old, newCfg)
	if err == nil {
		t.Error("Expected error when AI changed")
	}
}

func TestValidateImmutableFields_MetadateChanged(t *testing.T) {
	old := &PipelineConfig{
		Version:  "1.0",
		Name:     "test",
		Metadate: MetadataConfig{Type: "old"},
	}
	newCfg := &PipelineConfig{
		Version:  "1.0",
		Name:     "test",
		Metadate: MetadataConfig{Type: "new"},
	}

	err := validateImmutableFields(old, newCfg)
	if err == nil {
		t.Error("Expected error when Metadate changed")
	}
}

func TestValidateImmutableFields_NodesMutable(t *testing.T) {
	old := &PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Nodes:   map[string]NodeConfig{"A": {Name: "A"}},
	}
	newCfg := &PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Nodes:   map[string]NodeConfig{"B": {Name: "B"}},
	}

	err := validateImmutableFields(old, newCfg)
	if err != nil {
		t.Errorf("Nodes changes should be allowed, got: %v", err)
	}
}

func TestValidateImmutableFields_GraphMutable(t *testing.T) {
	old := &PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Graph:   "stateDiagram-v2\n    [*] --> A",
	}
	newCfg := &PipelineConfig{
		Version: "1.0",
		Name:    "test",
		Graph:   "stateDiagram-v2\n    [*] --> B",
	}

	err := validateImmutableFields(old, newCfg)
	if err != nil {
		t.Errorf("Graph changes should be allowed, got: %v", err)
	}
}
