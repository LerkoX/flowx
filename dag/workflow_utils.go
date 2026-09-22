package dag

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/executor"
)

// extractOutput 从节点输出中提取数据并保存到metadata
func (p *WorkflowImpl) extractOutput(ctx context.Context, node Node, stepResult *executor.StepResult, fullOutput string) error {
	// 获取节点配置
	nodeConfig := node.GetConfig()
	if nodeConfig == nil {
		return nil
	}

	// 检查是否有提取配置
	extractConfig, hasExtract := nodeConfig["extract"]
	if !hasExtract || extractConfig == nil {
		return nil
	}

	// 创建提取器
	extractor, err := p.createExtractor(extractConfig)
	if err != nil {
		return fmt.Errorf("failed to create extractor: %w", err)
	}

	// 执行提取
	extracted, err := extractor.Extract(fullOutput)
	if err != nil {
		return fmt.Errorf("failed to extract data from node %s: %w", node.Id(), err)
	}

	// 零提取护栏：节点声明了 extract（codec-block/regex）却一条都没提取到，通常意味着
	// 输出块被截断（如 docker exec 流中断，exec 364）或 extract 配置失效。不直接失败
	// （合法的空输出分支不应被误杀），但必须留下显式痕迹：日志告警 + metadata 标记
	// <node>.__extract_missing，供 Studio 提示"输出可能不完整"，并让下游绑定校验
	// （dag/binding_check.go）能给出定向上游的错误。
	if len(extracted) == 0 {
		markerKey := node.Id() + ".__extract_missing"
		fmt.Printf("Warning: node %s 声明了 extract 但未提取到任何数据"+
			"（输出被截断或 extract 配置失效）；标记 %s\n", node.Id(), markerKey)
		p.mu.Lock()
		if p.metadata == nil {
			p.metadata = make(Metadata)
		}
		p.metadata[markerKey] = core.FieldItem{
			Value:       "true",
			SrcNode:     node.Id(),
			Description: "节点声明了输出提取但未提取到数据（可能输出被截断或 extract 配置失效）",
		}
		p.mu.Unlock()
		if p.metadataStore != nil {
			if err := p.metadataStore.Set(ctx, markerKey, "true"); err != nil {
				fmt.Printf("Warning: Failed to save extract marker to store: %v\n", err)
			}
		}
		return nil
	}

	// 保存到 metadata（加锁防止并发写入）
	if len(extracted) > 0 {
		p.mu.Lock()
		defer p.mu.Unlock()

		// 确保 metadata 已初始化
		if p.metadata == nil {
			p.metadata = make(Metadata)
		}

		// 保存到内存 metadata（使用 FieldItem，设置 SrcNode）
		for key, fieldItem := range extracted {
			metadataKey := fmt.Sprintf("%s.%s", node.Id(), key)
			// 设置 SrcNode 为当前节点 ID
			fieldItem.SrcNode = node.Id()
			// 将 Value 转换为字符串存储，复杂类型序列化为 JSON
			fieldItem.Value = convertToString(core.GetValue(fieldItem.Value))
			p.metadata[metadataKey] = fieldItem
		}

		// 如果有 metadata store，同步保存
		if p.metadataStore != nil {
			for key, fieldItem := range extracted {
				metadataKey := fmt.Sprintf("%s.%s", node.Id(), key)
				valueStr := convertToString(core.GetValue(fieldItem.Value))
				if err := p.metadataStore.Set(ctx, metadataKey, valueStr); err != nil {
					fmt.Printf("Warning: Failed to save extracted data to store: %v\n", err)
				}
			}
		}
	}

	return nil
}

// convertToString 将值转换为字符串存储
// 复杂类型（slice、map）序列化为 JSON，简单类型直接转字符串
func convertToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []interface{}, map[string]interface{}:
		jsonBytes, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(jsonBytes)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// markStreamTruncated 标记节点输出流曾被截断（执行器已重挂/补齐，但可能仍缺字节）。
// 与 __extract_missing 同一套路：日志告警 + metadata 标记，供 Studio 提示"输出可能不完整"。
// 不改变节点状态：是否失败由执行器决定（重挂用尽时执行器已返回错误）。
func (p *WorkflowImpl) markStreamTruncated(ctx context.Context, node Node, stepName string) {
	markerKey := node.Id() + ".__stream_truncated"
	fmt.Printf("Warning: node %s 步骤 %s 的输出流曾被截断"+
		"（执行器已重挂/补齐，输出可能不完整）；标记 %s\n", node.Id(), stepName, markerKey)
	p.mu.Lock()
	if p.metadata == nil {
		p.metadata = make(Metadata)
	}
	p.metadata[markerKey] = core.FieldItem{
		Value:       "true",
		SrcNode:     node.Id(),
		Description: "执行器输出流曾被截断（docker exec 断流后已重挂/补齐，输出可能不完整）",
	}
	p.mu.Unlock()
	if p.metadataStore != nil {
		if err := p.metadataStore.Set(ctx, markerKey, "true"); err != nil {
			fmt.Printf("Warning: Failed to save stream truncation marker to store: %v\n", err)
		}
	}
}

// createExtractor 根据配置创建提取器
func (p *WorkflowImpl) createExtractor(extractConfig interface{}) (OutputExtractor, error) {
	if extractConfig == nil {
		return nil, nil
	}

	var configMap map[string]interface{}

	// 尝试解析为 map[string]interface{}
	if m, ok := extractConfig.(map[string]interface{}); ok {
		configMap = m
	} else if extractPtr, ok := extractConfig.(*core.ExtractConfig); ok && extractPtr != nil {
		// 转换 *core.ExtractConfig 到 map[string]interface{}
		configMap = make(map[string]interface{})
		if extractPtr.Type != "" {
			configMap["type"] = extractPtr.Type
		}
		if extractPtr.Patterns != nil {
			// 将 Patterns 转换为 map[string]interface{}
			patternsMap := make(map[string]interface{})
			for k, v := range extractPtr.Patterns {
				patternsMap[k] = v
			}
			configMap["patterns"] = patternsMap
		}
		if extractPtr.MaxOutputSize > 0 {
			configMap["maxOutputSize"] = extractPtr.MaxOutputSize
		}
	} else {
		return nil, fmt.Errorf("invalid extract config format")
	}

	// 获取提取类型
	extractType := "codec-block" // 默认类型
	if typeVal, ok := configMap["type"]; ok {
		if typeStr, ok := typeVal.(string); ok {
			extractType = typeStr
		}
	}

	// 获取输出大小限制
	maxOutputSize := 1024 * 1024 // 默认 1MB
	if sizeVal, ok := configMap["maxOutputSize"]; ok {
		if size, ok := sizeVal.(int); ok {
			maxOutputSize = size
		}
	}

	switch extractType {
	case "codec-block":
		return NewCodecBlockExtractor(maxOutputSize), nil

	case "regex":
		patterns := make(map[string]string)
		if patternsVal, ok := configMap["patterns"]; ok {
			if patternsMap, ok := patternsVal.(map[string]interface{}); ok {
				for k, v := range patternsMap {
					if patternStr, ok := v.(string); ok {
						patterns[k] = patternStr
					}
				}
			}
		}
		if len(patterns) == 0 {
			return nil, fmt.Errorf("regex extractor requires patterns")
		}
		return NewRegexExtractor(patterns, maxOutputSize)

	default:
		return nil, fmt.Errorf("unsupported extract type: %s", extractType)
	}
}