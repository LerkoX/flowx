package core

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// FieldItem 单个字段的完整定义
// 用于 Param 和 Metadata.Data 的每个字段
type FieldItem struct {
	Value       interface{} `yaml:"value"`        // 字段值
	Description string      `yaml:"description"`  // 字段描述（AI 生成时的参考）
	SrcNode     string      `yaml:"srcNode"`      // 来源节点ID（初始化为空）
}

// GetValue 获取字段值
// 兼容简单值和 FieldItem
func GetValue(v interface{}) interface{} {
	if fi, ok := v.(FieldItem); ok {
		return fi.Value
	}
	return v
}

// ConvertToFieldItem 将任意值转换为 FieldItem
// 支持向后兼容：简单值、完整格式、map 格式
func ConvertToFieldItem(v interface{}) FieldItem {
	switch val := v.(type) {
	case FieldItem:
		return val
	case map[string]interface{}:
		fi := FieldItem{}
		if desc, ok := val["description"].(string); ok {
			fi.Description = desc
		}
		if src, ok := val["srcNode"].(string); ok {
			fi.SrcNode = src
		}
		if val2, ok := val["value"]; ok {
			fi.Value = val2
		} else {
			fi.Value = val
		}
		return fi
	default:
		return FieldItem{Value: val}
	}
}

// UnmarshalYAML 实现 yaml.Unmarshaler 接口
// 支持两种格式：
// 1. 简单值: key: value  -> FieldItem{Value: value}
// 2. 完整格式: key: {value: xxx, description: yyy}
func (fi *FieldItem) UnmarshalYAML(node *yaml.Node) error {
	// 检查是否是简单值格式
	switch node.Kind {
	case yaml.ScalarNode:
		// 简单值：key: value
		// node.Value 已经是字符串形式
		// 需要解析为适当的类型
		var value interface{}
		if err := node.Decode(&value); err != nil {
			// 如果直接解码失败，作为字符串处理
			value = node.Value
		}
		*fi = FieldItem{Value: value}
		return nil

	case yaml.MappingNode:
		// 完整格式：key: {value: xxx, description: yyy}
		// 解码为 map[string]interface{}
		var data map[string]interface{}
		if err := node.Decode(&data); err != nil {
			return fmt.Errorf("failed to decode FieldItem: %w", err)
		}

		// 提取各字段
		if v, ok := data["value"]; ok {
			fi.Value = v
		}
		if desc, ok := data["description"].(string); ok {
			fi.Description = desc
		}
		if src, ok := data["srcNode"].(string); ok {
			fi.SrcNode = src
		}

		return nil

	default:
		// 其他类型，作为简单值处理
		var value interface{}
		if err := node.Decode(&value); err != nil {
			value = node.Value
		}
		*fi = FieldItem{Value: value}
		return nil
	}
}
