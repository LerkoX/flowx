package template

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/flosch/pongo2/v6"
)

func TestNewPongo2TemplateEngine(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	if engine == nil {
		t.Error("Expected non-nil template engine")
	}
}

func TestPongo2TemplateEngine_EvaluateBool_True(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{
		"status": "SUCCESS",
	}

	result, err := engine.EvaluateBool("{{ status == 'SUCCESS' }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !result {
		t.Error("Expected true for matching status")
	}
}

func TestPongo2TemplateEngine_EvaluateBool_False(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{
		"status": "FAILED",
	}

	result, err := engine.EvaluateBool("{{ status == 'SUCCESS' }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result {
		t.Error("Expected false for non-matching status")
	}
}

func TestPongo2TemplateEngine_EvaluateBool_EmptyResult(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{}

	// 当模板输出为空时，应该返回 false
	result, err := engine.EvaluateBool("{{ '' }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result {
		t.Error("Expected false for empty result")
	}
}

func TestPongo2TemplateEngine_EvaluateBool_StringTrue(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{}

	// 当模板输出 "true" 时，应该返回 true
	result, err := engine.EvaluateBool("{{ 'true' }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !result {
		t.Error("Expected true for 'true' string result")
	}
}

func TestPongo2TemplateEngine_EvaluateBool_StringFalse(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{}

	// 字符串字面量 "false" 是非空字符串，作为布尔值是 true
	// 这符合 Go/Pongo2 的语义
	result, err := engine.EvaluateBool("{{ 'false' }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !result {
		t.Error("Expected true for 'false' string (non-empty string is truthy)")
	}

	// 测试字符串与布尔字面量的比较（这才是正确的语义）
	result2, err := engine.EvaluateBool("{{ 'false' == false }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !result2 {
		t.Error("Expected true for 'false' == false comparison")
	}
}

func TestPongo2TemplateEngine_EvaluateBool_StringOne(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{}

	result, err := engine.EvaluateBool("{{ '1' }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !result {
		t.Error("Expected true for '1' string result")
	}
}

func TestPongo2TemplateEngine_EvaluateBool_StringZero(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{}

	// 字符串 "0" 是非空字符串，作为布尔值是 true
	result, err := engine.EvaluateBool("{{ '0' }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !result {
		t.Error("Expected true for '0' string (non-empty string is truthy)")
	}

	// 字符串 "0" 与字符串 "0" 比较是 true
	result2, err := engine.EvaluateBool("{{ '0' == '0' }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !result2 {
		t.Error("Expected true for '0' == '0' comparison")
	}
}

func TestPongo2TemplateEngine_EvaluateBool_NonEmptyString(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{}

	// 非空字符串应该返回 true
	result, err := engine.EvaluateBool("{{ 'hello' }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !result {
		t.Error("Expected true for non-empty string result")
	}
}

func TestPongo2TemplateEngine_EvaluateBool_InvalidSyntax(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{}

	_, err := engine.EvaluateBool("{{ unclosed tag", ctx)
	if err == nil {
		t.Error("Expected error for invalid template syntax")
	}
}

func TestPongo2TemplateEngine_EvaluateBool_ComplexCondition(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{
		"branch": "main",
		"status": "SUCCESS",
	}

	// 测试复杂条件
	result, err := engine.EvaluateBool("{{ branch == 'main' and status == 'SUCCESS' }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !result {
		t.Error("Expected true for matching complex condition")
	}
}

func TestPongo2TemplateEngine_EvaluateString(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{
		"name": "World",
	}

	result, err := engine.EvaluateString("{{ name }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result != "World" {
		t.Errorf("Expected 'World', got '%s'", result)
	}
}

func TestPongo2TemplateEngine_EvaluateString_WithSpaces(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{
		"text": "  hello world  ",
	}

	result, err := engine.EvaluateString("{{ text }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// 结果应该被 trim
	if result != "hello world" {
		t.Errorf("Expected trimmed 'hello world', got '%s'", result)
	}
}

func TestPongo2TemplateEngine_EvaluateString_InvalidSyntax(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{}

	_, err := engine.EvaluateString("{{ unclosed tag", ctx)
	if err == nil {
		t.Error("Expected error for invalid template syntax")
	}
}

func TestPongo2TemplateEngine_Validate_Valid(t *testing.T) {
	engine := NewPongo2TemplateEngine()

	err := engine.Validate("{{ status == 'SUCCESS' }}")
	if err != nil {
		t.Errorf("Unexpected error for valid expression: %v", err)
	}
}

func TestPongo2TemplateEngine_Validate_Invalid(t *testing.T) {
	engine := NewPongo2TemplateEngine()

	err := engine.Validate("{{ unclosed tag")
	if err == nil {
		t.Error("Expected error for invalid expression syntax")
	}
}

func TestPongo2TemplateEngine_Validate_Empty(t *testing.T) {
	engine := NewPongo2TemplateEngine()

	// 空字符串应该是有效的
	err := engine.Validate("")
	if err != nil {
		t.Errorf("Unexpected error for empty expression: %v", err)
	}
}

func TestPongo2TemplateEngine_EvaluateBool_WithPipelineData(t *testing.T) {
	engine := NewPongo2TemplateEngine()

	// 创建模拟的 pipeline 数据
	ctx := map[string]any{
		"pipelineId":     "pipe-123",
		"pipelineStatus": "RUNNING",
	}

	result, err := engine.EvaluateBool("{{ pipelineStatus == 'RUNNING' }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !result {
		t.Error("Expected true for matching pipeline status")
	}
}

func TestPongo2TemplateEngine_EvaluateBool_WithNodeData(t *testing.T) {
	engine := NewPongo2TemplateEngine()

	ctx := map[string]any{
		"nodeId":     "node-1",
		"nodeStatus": "SUCCESS",
	}

	result, err := engine.EvaluateBool("{{ nodeStatus == 'SUCCESS' }}", ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !result {
		t.Error("Expected true for matching node status")
	}
}

// ========== 过滤器单元测试 ==========

func TestFilterToJSON_String(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{"name": "hello"}

	result, err := engine.EvaluateString("{{ name | toJson }}", ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("Expected 'hello', got '%s'", result)
	}
}

func TestFilterToJSON_Map(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{"data": map[string]any{"key": "value", "num": 42}}

	result, err := engine.EvaluateString("{{ data | toJson }}", ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON result: %v", err)
	}
	if parsed["key"] != "value" {
		t.Errorf("Expected key='value', got %v", parsed["key"])
	}
}

func TestFilterToJSON_Slice(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{"items": []any{"a", "b", "c"}}

	result, err := engine.EvaluateString("{{ items | toJson }}", ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var parsed []any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON result: %v", err)
	}
	if len(parsed) != 3 {
		t.Errorf("Expected 3 items, got %d", len(parsed))
	}
}

func TestFilterToYaml(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{"data": map[string]any{"name": "test", "count": 42}}

	result, err := engine.EvaluateString("{{ data | toYaml }}", ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(result, "name: test") {
		t.Errorf("Expected YAML to contain 'name: test', got '%s'", result)
	}
}

func TestFilterToBase64(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{"text": "hello world"}

	result, err := engine.EvaluateString("{{ text | toBase64 }}", ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := base64.StdEncoding.EncodeToString([]byte("hello world"))
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestFilterToBase64_NonString(t *testing.T) {
	val := pongo2.AsValue(123)
	_, pongoErr := filterToBase64(val, nil)
	if pongoErr == nil {
		t.Error("Expected error for non-string input to toBase64")
	}
}

func TestFilterFromBase64(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	encoded := base64.StdEncoding.EncodeToString([]byte("hello world"))
	ctx := map[string]any{"encoded": encoded}

	result, err := engine.EvaluateString("{{ encoded | fromBase64 }}", ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", result)
	}
}

func TestFilterFromBase64_Invalid(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{"bad": "!!!not-base64!!!"}

	_, err := engine.EvaluateString("{{ bad | fromBase64 }}", ctx)
	if err == nil {
		t.Error("Expected error for invalid base64 input")
	}
}

func TestFilterURLEncode(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{"url": "hello world!"}

	result, err := engine.EvaluateString("{{ url | urlencode }}", ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != "hello+world%21" {
		t.Errorf("Expected 'hello+world%%21', got '%s'", result)
	}
}

func TestFilterURLEncode_NonString(t *testing.T) {
	// 直接测试 filter 函数
	val := pongo2.AsValue(123)
	_, pongoErr := filterURLEncode(val, nil)
	if pongoErr == nil {
		t.Error("Expected error for non-string input to urlencode")
	}
}

func TestFilterURLDecode(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{"encoded": "hello+world%21"}

	result, err := engine.EvaluateString("{{ encoded | urldecode }}", ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != "hello world!" {
		t.Errorf("Expected 'hello world!', got '%s'", result)
	}
}

func TestFilterURLDecode_Invalid(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{"bad": "%ZZ"}

	_, err := engine.EvaluateString("{{ bad | urldecode }}", ctx)
	if err == nil {
		t.Error("Expected error for invalid URL encoding")
	}
}

// TestPongo2TemplateEngine_EvaluateBool_DottedContextKeys 验证 context 含扁平点键时
// 条件表达式评估不再因 pongo2 键校验失败（续跑场景的历史 metadata 点键兜底过滤）
func TestPongo2TemplateEngine_EvaluateBool_DottedContextKeys(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{
		"iteration": 1,
		"b1_4.text": "分支1 第4步",
	}

	result, err := engine.EvaluateBool("{{ iteration < 3 }}", ctx)
	if err != nil {
		t.Fatalf("EvaluateBool should not fail with dotted context keys: %v", err)
	}
	if !result {
		t.Error("Expected true for '1 < 3'")
	}

	result, err = engine.EvaluateBool("{{ iteration < 3 }}", map[string]any{
		"iteration": 3,
		"b1_4.text": "x",
	})
	if err != nil {
		t.Fatalf("EvaluateBool should not fail with dotted context keys: %v", err)
	}
	if result {
		t.Error("Expected false for '3 < 3'")
	}
}

// TestPongo2TemplateEngine_EvaluateString_DottedContextKeys 验证含扁平点键的 context
// 不影响模板渲染，且嵌套结构中的同名值仍可正常引用
func TestPongo2TemplateEngine_EvaluateString_DottedContextKeys(t *testing.T) {
	engine := NewPongo2TemplateEngine()
	ctx := map[string]any{
		"b1_4.text": "扁平死键",
		"b1_4":      map[string]any{"text": "嵌套值"},
		"city":      "深圳",
	}

	result, err := engine.EvaluateString("{{ b1_4.text }} - {{ city }}", ctx)
	if err != nil {
		t.Fatalf("EvaluateString should not fail with dotted context keys: %v", err)
	}
	if result != "嵌套值 - 深圳" {
		t.Errorf("Expected '嵌套值 - 深圳', got '%s'", result)
	}
}
