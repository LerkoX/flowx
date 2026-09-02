package dag

import (
	"github.com/LerkoX/flowx/core"
)

// DGAEvaluationContext 是EvaluationContext接口的实现
type DGAEvaluationContext struct {
	data      map[string]any
	node      Node
	pipeline  Pipeline
	iteration int
}

// NewEvaluationContext 创建一个新的求值上下文
func NewEvaluationContext() EvaluationContext {
	return &DGAEvaluationContext{
		data: make(map[string]any),
	}
}

// Get 从上下文中获取值
func (c *DGAEvaluationContext) Get(key string) (any, bool) {
	val, ok := c.data[key]
	return val, ok
}

// convertBoolToString 递归地将布尔值转换为字符串
func convertBoolToString(v any) any {
	switch val := v.(type) {
	case bool:
		if val {
			return "true"
		}
		return "false"
	case map[string]any:
		result := make(map[string]any)
		for k, v := range val {
			result[k] = convertBoolToString(v)
		}
		return result
	case map[interface{}]interface{}:
		result := make(map[string]any)
		for k, v := range val {
			if key, ok := k.(string); ok {
				result[key] = convertBoolToString(v)
			}
		}
		return result
	default:
		return v
	}
}

// putContextValue 将键值放入 context：点键（NodeID.key）转换为嵌套结构，
// 普通键直接设置。pongo2 要求 context 键为合法标识符，扁平点键不可达且会导致
// 其校验致命失败，因此任何进入 context 的数据都必须经过此转换
func putContextValue(result map[string]any, k string, v any) {
	if dotIdx := lastIndexOfByte(k, '.'); dotIdx > 0 {
		nodeID := k[:dotIdx]
		keyName := k[dotIdx+1:]
		// 如果节点ID 的嵌套对象不存在，创建它
		if _, exists := result[nodeID]; !exists {
			result[nodeID] = make(map[string]any)
		}
		// 将键值添加到节点的嵌套对象中
		if nodeObj, ok := result[nodeID].(map[string]any); ok {
			nodeObj[keyName] = v
		}
	} else {
		result[k] = v
	}
}

// All 返回上下文中所有数据的副本
// 合并了：基础数据、节点数据、流水线数据
// 将 "NodeID.key" 格式的扁平键转换为嵌套结构
func (c *DGAEvaluationContext) All() map[string]any {
	result := make(map[string]any)

	// 复制基础数据（点键转换为嵌套结构：续跑场景下历史 metadata 的扁平点键
	// 会经 WithParams 进入基础数据，直接拷贝会导致 pongo2 键校验失败）
	for k, v := range c.data {
		putContextValue(result, k, v)
	}

	// 添加节点相关数据
	if c.node != nil {
		result["nodeId"] = c.node.Id()
		result["nodeStatus"] = c.node.Status()
	}

	// 添加迭代计数器
	result["iteration"] = c.iteration

	// 添加流水线相关数据
	if c.pipeline != nil {
		result["pipelineId"] = c.pipeline.Id()
		result["pipelineStatus"] = c.pipeline.Status()

		// 将 Param 添加到上下文中，使其可以直接访问
		if pipelineImpl, ok := c.pipeline.(*PipelineImpl); ok {
			if len(pipelineImpl.param) > 0 {
				paramValues := make(map[string]any)
				for k, v := range pipelineImpl.param {
					val := core.GetValue(v.Value)
					result[k] = val
					paramValues[k] = val
				}
				// 同时提供 Param.xxx 的访问方式
				result["Param"] = convertBoolToString(paramValues)
			}
		}

		// 添加 metadata，并将 "NodeID.key" 格式转换为嵌套结构
		if metadata := c.pipeline.Metadata(); metadata != nil {
			for k, v := range metadata {
				putContextValue(result, k, core.GetValue(v.Value))
			}
		}
	}

	// 转换所有布尔值为字符串
	for k, v := range result {
		converted := convertBoolToString(v)
		result[k] = converted
	}

	return result
}

// lastIndexOf 返回字符串中最后一个点的索引
func lastIndexOf(s string, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// lastIndexOfByte 返回字符串中最后一个指定字节的索引
func lastIndexOfByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// WithNode 设置当前节点并返回新的上下文（链式调用）
func (c *DGAEvaluationContext) WithNode(node Node) EvaluationContext {
	newCtx := &DGAEvaluationContext{
		data:      make(map[string]any),
		node:      node,
		pipeline:  c.pipeline,
		iteration: c.iteration,
	}
	for k, v := range c.data {
		newCtx.data[k] = v
	}
	return newCtx
}

// WithPipeline 设置流水线并返回新的上下文（链式调用）
func (c *DGAEvaluationContext) WithPipeline(pipeline Pipeline) EvaluationContext {
	newCtx := &DGAEvaluationContext{
		data:      make(map[string]any),
		node:      c.node,
		pipeline:  pipeline,
		iteration: c.iteration,
	}
	for k, v := range c.data {
		newCtx.data[k] = v
	}
	return newCtx
}

// WithParams 添加参数到上下文并返回新的上下文（链式调用）
func (c *DGAEvaluationContext) WithParams(params map[string]any) EvaluationContext {
	newCtx := &DGAEvaluationContext{
		data:      make(map[string]any),
		node:      c.node,
		pipeline:  c.pipeline,
		iteration: c.iteration,
	}
	for k, v := range c.data {
		newCtx.data[k] = v
	}
	for k, v := range params {
		newCtx.data[k] = v
	}
	return newCtx
}

// WithIteration 设置迭代计数器并返回新的上下文（链式调用）
func (c *DGAEvaluationContext) WithIteration(iteration int) EvaluationContext {
	newCtx := &DGAEvaluationContext{
		data:      make(map[string]any),
		node:      c.node,
		pipeline:  c.pipeline,
		iteration: iteration,
	}
	for k, v := range c.data {
		newCtx.data[k] = v
	}
	return newCtx
}

// Iteration 返回当前迭代计数器值
func (c *DGAEvaluationContext) Iteration() int {
	return c.iteration
}
