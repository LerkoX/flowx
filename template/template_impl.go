package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flosch/pongo2/v6"
)

// 预检查Pongo2TemplateEngine是否实现了TemplateEngine接口
var _ TemplateEngine = (*Pongo2TemplateEngine)(nil)

// 注册自定义过滤器
func init() {
	pongo2.RegisterFilter("tojson", filterToJSON)
}

// filterToJSON 将值转换为 JSON 字符串
func filterToJSON(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	var result string

	switch v := in.Interface().(type) {
	case string:
		// 字符串类型，直接返回
		result = v
	case []interface{}:
		// 切片/数组，转换为 JSON 数组
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, &pongo2.Error{
				Sender:    "filter:tojson",
				OrigError: err,
			}
		}
		result = string(jsonBytes)
	case map[string]interface{}:
		// Map，转换为 JSON 对象
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, &pongo2.Error{
				Sender:    "filter:tojson",
				OrigError: err,
			}
		}
		result = string(jsonBytes)
	default:
		// 其他类型，先转换为 interface{} 再 Marshal
		jsonBytes, err := json.Marshal(in.Interface())
		if err != nil {
			return nil, &pongo2.Error{
				Sender:    "filter:tojson",
				OrigError: err,
			}
		}
		result = string(jsonBytes)
	}

	return pongo2.AsSafeValue(result), nil
}

// Pongo2TemplateEngine 使用pongo2作为模板引擎的实现
type Pongo2TemplateEngine struct{}

// NewPongo2TemplateEngine 创建一个新的Pongo2模板引擎实例
func NewPongo2TemplateEngine() TemplateEngine {
	return &Pongo2TemplateEngine{}
}

// EvaluateBool 评估模板表达式，返回布尔值
func (e *Pongo2TemplateEngine) EvaluateBool(expression string, ctx map[string]any) (bool, error) {
	// 使用 if-else 形式评估布尔表达式
	// 如果表达式包含 {{ }}，则去除外层
	innerExpr := expression
	if strings.HasPrefix(expression, "{{") && strings.HasSuffix(expression, "}}") {
		innerExpr = strings.TrimSpace(expression[2 : len(expression)-2])
	}

	// 构造 if-else 模板
	// 将布尔字面量替换为字符串字面量
	boolProcessedExpr := strings.ReplaceAll(innerExpr, " true ", " 'true' ")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, " false ", " 'false' ")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "== true", "== 'true'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "== false", "== 'false'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "!= true", "!= 'true'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "!= false", "!= 'false'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "> true", "> 'true'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "< true", "< 'true'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, ">= true", ">= 'true'")
	boolProcessedExpr = strings.ReplaceAll(boolProcessedExpr, "<= true", "<= 'true'")

	ifTemplate := "{% if " + boolProcessedExpr + " %}true{% else %}false{% endif %}"
	template, err := pongo2.FromString(ifTemplate)
	if err != nil {
		return false, fmt.Errorf("failed to parse expression '%s': %w", expression, err)
	}

	// 执行模板
	result, err := template.Execute(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to execute expression '%s': %w", expression, err)
	}

	// 将结果转换为布尔值
	result = strings.TrimSpace(result)
	return strings.ToLower(result) == "true", nil
}

// EvaluateString 评估模板表达式，返回字符串
func (e *Pongo2TemplateEngine) EvaluateString(expression string, ctx map[string]any) (string, error) {
	tmpl, err := pongo2.FromString(expression)
	if err != nil {
		return "", fmt.Errorf("failed to parse expression '%s': %w", expression, err)
	}

	result, err := tmpl.Execute(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to execute expression '%s': %w", expression, err)
	}

	return strings.TrimSpace(result), nil
}

// Validate 验证表达式语法是否正确
func (e *Pongo2TemplateEngine) Validate(expression string) error {
	_, err := pongo2.FromString(expression)
	if err != nil {
		return fmt.Errorf("invalid expression syntax: %w", expression, err)
	}
	return nil
}
