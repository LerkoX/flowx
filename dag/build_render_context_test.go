package dag

import (
	"testing"

	"github.com/LerkoX/flowx/core"
)

// TestBuildRenderContext_Empty 测试空上下文
func TestBuildRenderContext_Empty(t *testing.T) {
	workflow := NewWorkflow(nil)
	impl := workflow.(*WorkflowImpl)

	// 确保 param 和 metadata 都是 nil/empty
	impl.mu.Lock()
	impl.param = nil
	impl.metadata = nil
	impl.mu.Unlock()

	ctx := impl.buildRenderContext()

	if ctx == nil {
		t.Fatal("buildRenderContext should not return nil")
	}

	// 空 workflow 可能会有一个空的 Metadata map
	// 这是实现的行为，不应该 panic
	if ctx["Metadata"] != nil {
		metadataMap, ok := ctx["Metadata"].(map[string]any)
		if ok && len(metadataMap) > 0 {
			t.Errorf("Empty workflow should have empty Metadata, got %d items", len(metadataMap))
		}
	}
}

// TestBuildRenderContext_WithParam 测试带 Param 的上下文
func TestBuildRenderContext_WithParam(t *testing.T) {
	workflow := NewWorkflow(nil)
	impl := workflow.(*WorkflowImpl)

	// 设置 Param
	impl.SetParam(map[string]interface{}{
		"env":     "production",
		"version": "v1.0.0",
		"count":   42,
	})

	ctx := impl.buildRenderContext()

	// 检查顶层展开
	if ctx["env"] != "production" {
		t.Errorf("ctx[env] = %v, want production", ctx["env"])
	}
	if ctx["version"] != "v1.0.0" {
		t.Errorf("ctx[version] = %v, want v1.0.0", ctx["version"])
	}
	if ctx["count"] != 42 {
		t.Errorf("ctx[count] = %v, want 42", ctx["count"])
	}

	// 检查 Param map
	param, ok := ctx["Param"].(map[string]any)
	if !ok {
		t.Fatal("ctx[Param] should be map[string]any")
	}
	if param["env"] != "production" {
		t.Errorf("Param[env] = %v, want production", param["env"])
	}
}

// TestBuildRenderContext_WithParam_Boolean 测试带布尔类型 Param 的上下文
func TestBuildRenderContext_WithParam_Boolean(t *testing.T) {
	workflow := NewWorkflow(nil)
	impl := workflow.(*WorkflowImpl)

	// 设置带布尔值的 Param
	impl.SetParam(map[string]interface{}{
		"enabled": true,
		"debug":   false,
	})

	ctx := impl.buildRenderContext()

	// 检查布尔值（FieldItem.Value 返回原始类型）
	if ctx["enabled"] != true {
		t.Errorf("ctx[enabled] = %v, want true", ctx["enabled"])
	}
	if ctx["debug"] != false {
		t.Errorf("ctx[debug] = %v, want false", ctx["debug"])
	}
}

// TestBuildRenderContext_WithMetadata 测试带 Metadata 的上下文
func TestBuildRenderContext_WithMetadata(t *testing.T) {
	workflow := NewWorkflow(nil)
	impl := workflow.(*WorkflowImpl)

	// 设置 Metadata
	impl.mu.Lock()
	impl.metadata = Metadata{
		"Build.buildId":    core.FieldItem{Value: "12345", SrcNode: "Build"},
		"Build.status":     core.FieldItem{Value: "success", SrcNode: "Build"},
		"Deploy.namespace": core.FieldItem{Value: "prod", SrcNode: "Deploy"},
	}
	impl.mu.Unlock()

	ctx := impl.buildRenderContext()

	// 检查 Metadata map
	metadata, ok := ctx["Metadata"].(map[string]any)
	if !ok {
		t.Fatal("ctx[Metadata] should be map[string]any")
	}

	// 检查平铺的键
	if metadata["Build.buildId"] != "12345" {
		t.Errorf("Metadata[Build.buildId] = %v, want 12345", metadata["Build.buildId"])
	}

	// 检查嵌套结构
	build, ok := ctx["Build"].(map[string]any)
	if !ok {
		t.Fatal("ctx[Build] should be map[string]any")
	}
	if build["buildId"] != "12345" {
		t.Errorf("Build.buildId = %v, want 12345", build["buildId"])
	}
	if build["status"] != "success" {
		t.Errorf("Build.status = %v, want success", build["status"])
	}

	deploy, ok := ctx["Deploy"].(map[string]any)
	if !ok {
		t.Fatal("ctx[Deploy] should be map[string]any")
	}
	if deploy["namespace"] != "prod" {
		t.Errorf("Deploy.namespace = %v, want prod", deploy["namespace"])
	}
}

// TestBuildRenderContext_WithMetadata_StandaloneKey 测试 Metadata 中不带点的键
func TestBuildRenderContext_WithMetadata_StandaloneKey(t *testing.T) {
	workflow := NewWorkflow(nil)
	impl := workflow.(*WorkflowImpl)

	// 设置 Metadata（包含不带点的键）
	impl.mu.Lock()
	impl.metadata = Metadata{
		"globalKey":     core.FieldItem{Value: "globalValue", SrcNode: ""},
		"Build.buildId": core.FieldItem{Value: "12345", SrcNode: "Build"},
	}
	impl.mu.Unlock()

	ctx := impl.buildRenderContext()

	// 不带点的键应该直接添加
	if ctx["globalKey"] != "globalValue" {
		t.Errorf("ctx[globalKey] = %v, want globalValue", ctx["globalKey"])
	}

	// 嵌套的键应该只能通过嵌套结构访问
	build, ok := ctx["Build"].(map[string]any)
	if !ok {
		t.Fatal("ctx[Build] should exist as nested structure")
	}
	if build["buildId"] != "12345" {
		t.Errorf("Build.buildId = %v, want 12345", build["buildId"])
	}
}

// TestBuildRenderContext_ParamAndMetadata 测试同时有 Param 和 Metadata
func TestBuildRenderContext_ParamAndMetadata(t *testing.T) {
	workflow := NewWorkflow(nil)
	impl := workflow.(*WorkflowImpl)

	// 设置 Param
	impl.SetParam(map[string]interface{}{
		"env": "staging",
	})

	// 设置 Metadata
	impl.mu.Lock()
	impl.metadata = Metadata{
		"Build.status": core.FieldItem{Value: "running", SrcNode: "Build"},
	}
	impl.mu.Unlock()

	ctx := impl.buildRenderContext()

	// 两者都应该存在
	if ctx["env"] != "staging" {
		t.Errorf("ctx[env] = %v, want staging", ctx["env"])
	}

	build, ok := ctx["Build"].(map[string]any)
	if !ok {
		t.Fatal("ctx[Build] should exist")
	}
	if build["status"] != "running" {
		t.Errorf("Build.status = %v, want running", build["status"])
	}

	// Metadata map 也应该存在
	var ok2 bool
	_, ok2 = ctx["Metadata"].(map[string]any)
	if !ok2 {
		t.Fatal("ctx[Metadata] should exist")
	}
}

// TestBuildRenderContext_ParamNil 测试 Param 为 nil 的情况
func TestBuildRenderContext_ParamNil(t *testing.T) {
	workflow := NewWorkflow(nil)
	impl := workflow.(*WorkflowImpl)

	// 不设置 Param，保持 nil
	ctx := impl.buildRenderContext()

	// 应该返回空 map，而不是 panic
	if ctx == nil {
		t.Fatal("buildRenderContext should not return nil")
	}

	// 不应该有 Param 键
	if _, exists := ctx["Param"]; exists {
		t.Error("Empty workflow should not have Param in context")
	}
}

// TestBuildRenderContext_MetadataNil 测试 Metadata 为 nil 的情况
func TestBuildRenderContext_MetadataNil(t *testing.T) {
	workflow := NewWorkflow(nil)
	impl := workflow.(*WorkflowImpl)

	// 确保 metadata 是 nil
	impl.mu.Lock()
	impl.metadata = nil
	impl.mu.Unlock()

	ctx := impl.buildRenderContext()

	// 应该返回空 map，而不是 panic
	if ctx == nil {
		t.Fatal("buildRenderContext should not return nil")
	}

	// Metadata() 可能返回空 map 而不是 nil
	metadata := ctx["Metadata"]
	if metadata != nil {
		metadataMap, ok := metadata.(map[string]any)
		if ok && len(metadataMap) > 0 {
			t.Error("Workflow without metadata should have empty Metadata in context")
		}
	}
}

// TestBuildRenderContext_Concurrent 测试并发安全性
func TestBuildRenderContext_Concurrent(t *testing.T) {
	workflow := NewWorkflow(nil)
	impl := workflow.(*WorkflowImpl)

	// 设置初始 Param
	impl.SetParam(map[string]interface{}{
		"initial": "value",
	})

	// 并发读取
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			ctx := impl.buildRenderContext()
			if ctx == nil {
				t.Error("buildRenderContext returned nil")
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestBuildRenderContext_FieldItemValue 测试 FieldItem 值的提取
func TestBuildRenderContext_FieldItemValue(t *testing.T) {
	workflow := NewWorkflow(nil)
	impl := workflow.(*WorkflowImpl)

	// 直接设置 FieldItem 类型的 Param
	impl.mu.Lock()
	impl.param = map[string]core.FieldItem{
		"stringVal": core.FieldItem{Value: "hello"},
		"intVal":    core.FieldItem{Value: 42},
		"boolVal":   core.FieldItem{Value: true},
		"floatVal":  core.FieldItem{Value: 3.14},
		"mapVal":    core.FieldItem{Value: map[string]any{"key": "value"}},
	}
	impl.mu.Unlock()

	ctx := impl.buildRenderContext()

	// 验证各种类型的值都被正确提取
	if ctx["stringVal"] != "hello" {
		t.Errorf("stringVal = %v, want hello", ctx["stringVal"])
	}
	if ctx["intVal"] != 42 {
		t.Errorf("intVal = %v, want 42", ctx["intVal"])
	}
	if ctx["boolVal"] != true {
		t.Errorf("boolVal = %v, want true", ctx["boolVal"])
	}
	if ctx["floatVal"] != 3.14 {
		t.Errorf("floatVal = %v, want 3.14", ctx["floatVal"])
	}

	mapVal, ok := ctx["mapVal"].(map[string]any)
	if !ok {
		t.Fatal("mapVal should be map[string]any")
	}
	if mapVal["key"] != "value" {
		t.Errorf("mapVal[key] = %v, want value", mapVal["key"])
	}
}

// ========== tryParseJSON 单元测试 ==========

func TestTryParseJSON_NonString(t *testing.T) {
	input := 42
	result := tryParseJSON(input)
	if result != input {
		t.Errorf("Expected non-string to be returned as-is, got %v", result)
	}
}

func TestTryParseJSON_EmptyString(t *testing.T) {
	input := ""
	result := tryParseJSON(input)
	if result != input {
		t.Errorf("Expected empty string to be returned as-is, got %v", result)
	}
}

func TestTryParseJSON_SingleChar(t *testing.T) {
	input := "x"
	result := tryParseJSON(input)
	if result != input {
		t.Errorf("Expected single char string to be returned as-is, got %v", result)
	}
}

func TestTryParseJSON_NotJSON(t *testing.T) {
	input := "hello world"
	result := tryParseJSON(input)
	if result != input {
		t.Errorf("Expected non-JSON string to be returned as-is, got %v", result)
	}
}

func TestTryParseJSON_InvalidJSON(t *testing.T) {
	input := "{invalid json}"
	result := tryParseJSON(input)
	if result != input {
		t.Errorf("Expected invalid JSON string to be returned as-is, got %v", result)
	}
}

func TestTryParseJSON_Object(t *testing.T) {
	input := `{"name":"test","count":42}`
	result := tryParseJSON(input)

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", result)
	}
	if m["name"] != "test" {
		t.Errorf("Expected name='test', got %v", m["name"])
	}
}

func TestTryParseJSON_Array(t *testing.T) {
	input := `["a","b","c"]`
	result := tryParseJSON(input)

	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}
	if len(arr) != 3 {
		t.Errorf("Expected 3 items, got %d", len(arr))
	}
}

func TestTryParseJSON_NestedObject(t *testing.T) {
	input := `{"outer":{"inner":"value"}}`
	result := tryParseJSON(input)

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", result)
	}
	outer, ok := m["outer"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected nested map, got %T", m["outer"])
	}
	if outer["inner"] != "value" {
		t.Errorf("Expected inner='value', got %v", outer["inner"])
	}
}

func TestTryParseJSON_Whitespace(t *testing.T) {
	input := `   {"key":"val"}   `
	result := tryParseJSON(input)

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", result)
	}
	if m["key"] != "val" {
		t.Errorf("Expected key='val', got %v", m["key"])
	}
}

func TestTryParseJSON_BracketsButNotJSON(t *testing.T) {
	input := "[not json]"
	result := tryParseJSON(input)
	if result != input {
		t.Errorf("Expected brackets-but-not-JSON string to be returned as-is, got %v", result)
	}
}

func TestTryParseJSON_Boolean(t *testing.T) {
	input := "true"
	result := tryParseJSON(input)
	if result != input {
		t.Errorf("Expected boolean string to be returned as-is, got %v", result)
	}
}

func TestTryParseJSON_Null(t *testing.T) {
	input := "null"
	result := tryParseJSON(input)
	if result != input {
		t.Errorf("Expected null string to be returned as-is, got %v", result)
	}
}
