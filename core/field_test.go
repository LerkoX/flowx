package core

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGetValue(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{
			name:  "FieldItem with string value",
			input: FieldItem{Value: "v1.0.0", Description: "版本", SrcNode: "node1"},
			want:  "v1.0.0",
		},
		{
			name:  "FieldItem with number value",
			input: FieldItem{Value: 42, Description: "数量", SrcNode: ""},
			want:  42,
		},
		{
			name:  "FieldItem with bool value",
			input: FieldItem{Value: true, Description: "", SrcNode: ""},
			want:  true,
		},
		{
			name:  "FieldItem with map value",
			input: FieldItem{Value: map[string]interface{}{"key": "val"}},
			want:  map[string]interface{}{"key": "val"},
		},
		{
			name:  "Plain string (not FieldItem)",
			input: "plain string",
			want:  "plain string",
		},
		{
			name:  "Plain number",
			input: 123,
			want:  123,
		},
		{
			name:  "Plain bool",
			input: false,
			want:  false,
		},
		{
			name:  "Nil value",
			input: nil,
			want:  nil,
		},
		{
			name:  "Empty FieldItem",
			input: FieldItem{},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetValue(tt.input)
			// Use reflect.DeepEqual for complex types, string comparison for primitives
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToFieldItem(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  FieldItem
	}{
		{
			name:  "Already FieldItem",
			input: FieldItem{Value: "test", Description: "desc", SrcNode: "node1"},
			want:  FieldItem{Value: "test", Description: "desc", SrcNode: "node1"},
		},
		{
			name:  "Simple string value",
			input: "v1.0.0",
			want:  FieldItem{Value: "v1.0.0"},
		},
		{
			name:  "Simple number value",
			input: 42,
			want:  FieldItem{Value: 42},
		},
		{
			name:  "Simple boolean value",
			input: true,
			want:  FieldItem{Value: true},
		},
		{
			name:  "Map with value field only",
			input: map[string]interface{}{"value": "test-value"},
			want:  FieldItem{Value: "test-value"},
		},
		{
			name:  "Map with all fields",
			input: map[string]interface{}{
				"value":       "full-value",
				"description": "完整描述",
				"srcNode":     "source-node",
			},
			want: FieldItem{
				Value:       "full-value",
				Description: "完整描述",
				SrcNode:     "source-node",
			},
		},
		{
			name:  "Map with partial fields",
			input: map[string]interface{}{
				"value":       123,
				"description": "数字值",
			},
			want: FieldItem{
				Value:       123,
				Description: "数字值",
			},
		},
		{
			name:  "Map with nested object as value",
			input: map[string]interface{}{"nested": "data"},
			want:  FieldItem{Value: map[string]interface{}{"nested": "data"}},
		},
		{
			name:  "Nil input",
			input: nil,
			want:  FieldItem{Value: nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertToFieldItem(tt.input)

			// Compare Value
			if !reflect.DeepEqual(got.Value, tt.want.Value) {
				t.Errorf("ConvertToFieldItem().Value = %v, want %v", got.Value, tt.want.Value)
			}

			if got.Description != tt.want.Description {
				t.Errorf("ConvertToFieldItem().Description = %v, want %v", got.Description, tt.want.Description)
			}
			if got.SrcNode != tt.want.SrcNode {
				t.Errorf("ConvertToFieldItem().SrcNode = %v, want %v", got.SrcNode, tt.want.SrcNode)
			}
		})
	}
}

func TestFieldItemUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    FieldItem
		wantErr bool
	}{
		{
			name: "Simple string value",
			yaml: `value: "v1.0.0"`,
			want: FieldItem{Value: "v1.0.0"},
		},
		{
			name: "Simple number value",
			yaml: `value: 42`,
			want: FieldItem{Value: 42},
		},
		{
			name: "Simple boolean true",
			yaml: `value: true`,
			want: FieldItem{Value: true},
		},
		{
			name: "Simple boolean false",
			yaml: `value: false`,
			want: FieldItem{Value: false},
		},
		{
			name: "Full format with all fields",
			yaml: `
value: "full-value"
description: "完整描述"
srcNode: "source-node"
`,
			want: FieldItem{
				Value:       "full-value",
				Description: "完整描述",
				SrcNode:     "source-node",
			},
		},
		{
			name: "Full format partial fields",
			yaml: `
value: 123
description: "数字值"
`,
			want: FieldItem{
				Value:       123,
				Description: "数字值",
			},
		},
		{
			name: "Just description",
			yaml: `
value: "test"
description: "描述"
`,
			want: FieldItem{
				Value:       "test",
				Description: "描述",
			},
		},
		{
			name: "List value",
			yaml: `
value:
  - item1
  - item2
  - item3
`,
			want: FieldItem{
				Value: []interface{}{"item1", "item2", "item3"},
			},
		},
		{
			name: "Map value",
			yaml: `
value:
  key1: val1
  key2: val2
`,
			want: FieldItem{
				Value: map[string]interface{}{"key1": "val1", "key2": "val2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fi FieldItem
			err := yaml.Unmarshal([]byte(tt.yaml), &fi)

			if tt.wantErr {
				if err == nil {
					t.Error("UnmarshalYAML() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("UnmarshalYAML() error = %v", err)
			}

			if !reflect.DeepEqual(fi.Value, tt.want.Value) {
				t.Errorf("UnmarshalYAML().Value = %v, want %v", fi.Value, tt.want.Value)
			}
			if fi.Description != tt.want.Description {
				t.Errorf("UnmarshalYAML().Description = %v, want %v", fi.Description, tt.want.Description)
			}
			if fi.SrcNode != tt.want.SrcNode {
				t.Errorf("UnmarshalYAML().SrcNode = %v, want %v", fi.SrcNode, tt.want.SrcNode)
			}
		})
	}
}

func TestFieldItemUnmarshalYAML_SimpleValue(t *testing.T) {
	// Test that simple value format works: just a value without field names
	tests := []struct {
		name    string
		yaml    string
		wantVal interface{}
	}{
		{
			name:    "Simple quoted string",
			yaml:    `"simple string"`,
			wantVal: "simple string",
		},
		{
			name:    "Simple number",
			yaml:    `42`,
			wantVal: 42,
		},
		{
			name:    "Simple float",
			yaml:    `3.14`,
			wantVal: 3.14,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fi FieldItem
			err := yaml.Unmarshal([]byte(tt.yaml), &fi)
			if err != nil {
				t.Fatalf("UnmarshalYAML() error = %v", err)
			}

			if !reflect.DeepEqual(fi.Value, tt.wantVal) {
				t.Errorf("UnmarshalYAML().Value = %v, want %v", fi.Value, tt.wantVal)
			}
		})
	}
}
