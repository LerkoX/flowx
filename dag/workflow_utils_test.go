package dag

import (
	"encoding/json"
	"testing"
)

// TestConvertToString_String 测试字符串类型直接返回
func TestConvertToString_String(t *testing.T) {
	input := "hello"
	result := convertToString(input)
	if result != "hello" {
		t.Errorf("Expected 'hello', got '%s'", result)
	}
}

// TestConvertToString_Int 测试整数类型
func TestConvertToString_Int(t *testing.T) {
	input := 42
	result := convertToString(input)
	if result != "42" {
		t.Errorf("Expected '42', got '%s'", result)
	}
}

// TestConvertToString_Float 测试浮点数类型
func TestConvertToString_Float(t *testing.T) {
	input := 3.14
	result := convertToString(input)
	if result != "3.14" {
		t.Errorf("Expected '3.14', got '%s'", result)
	}
}

// TestConvertToString_Bool 测试布尔类型
func TestConvertToString_Bool(t *testing.T) {
	input := true
	result := convertToString(input)
	if result != "true" {
		t.Errorf("Expected 'true', got '%s'", result)
	}
}

// TestConvertToString_Slice 测试切片类型序列化为 JSON
func TestConvertToString_Slice(t *testing.T) {
	input := []interface{}{"a", "b", "c"}
	result := convertToString(input)

	var parsed []interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	if len(parsed) != 3 {
		t.Errorf("Expected 3 items, got %d", len(parsed))
	}
}

// TestConvertToString_Map 测试 Map 类型序列化为 JSON
func TestConvertToString_Map(t *testing.T) {
	input := map[string]interface{}{"key": "value", "num": 42}
	result := convertToString(input)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	if parsed["key"] != "value" {
		t.Errorf("Expected key='value', got %v", parsed["key"])
	}
}

// TestConvertToString_Nil 测试 nil 值
func TestConvertToString_Nil(t *testing.T) {
	result := convertToString(nil)
	if result != "<nil>" {
		t.Errorf("Expected '<nil>', got '%s'", result)
	}
}

// TestConvertToString_NestedMap 测试嵌套 Map
func TestConvertToString_NestedMap(t *testing.T) {
	input := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": "value",
		},
	}
	result := convertToString(input)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	outer, ok := parsed["outer"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected nested map")
	}
	if outer["inner"] != "value" {
		t.Errorf("Expected inner='value', got %v", outer["inner"])
	}
}

// TestConvertToString_MixedSlice 测试混合类型切片
func TestConvertToString_MixedSlice(t *testing.T) {
	input := []interface{}{"str", 42, true, map[string]interface{}{"key": "val"}}
	result := convertToString(input)

	var parsed []interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	if len(parsed) != 4 {
		t.Errorf("Expected 4 items, got %d", len(parsed))
	}
}
