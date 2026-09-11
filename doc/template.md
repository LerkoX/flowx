# 模板引擎

本文档介绍 Workflowx 的模板引擎机制，包括 pongo2 引擎的使用、Param 自引用渲染、Metadata 渲染以及运行时命令渲染。

## TemplateEngine 接口

```go
type TemplateEngine interface {
    EvaluateBool(expression string, ctx map[string]any) (bool, error)    // 评估布尔表达式
    EvaluateString(expression string, ctx map[string]any) (string, error) // 渲染字符串模板
    Validate(expression string) error                                    // 校验语法
}
```

## Pongo2TemplateEngine

基于 [pongo2](https://github.com/flosch/pongo2)（Django 模板语法）的实现。

```go
engine := NewPongo2TemplateEngine()
```

### EvaluateBool

将表达式包装为 `{% if ... %}true{% else %}false{% endif %}` 进行评估。

```go
result, err := engine.EvaluateBool("Param.env == \"production\"", ctx)
// result == true 或 false
```

**布尔值处理：**

pongo2 中的布尔字面量会被自动转换为字符串等价形式，确保与上下文中的字符串值正确比较。

### EvaluateString

直接渲染 pongo2 模板。

```go
result, err := engine.EvaluateString("Hello {{ Param.name }}", ctx)
// result == "Hello world"（如果 Param.name == "world"）
```

### Validate

检查表达式语法是否有效。

```go
err := engine.Validate("{{ Param.env == ")
// err != nil（语法错误）
```

## 渲染阶段

模板渲染在两个不同阶段发生：

### 1. 配置解析阶段（Workflow 启动前）

在 `RuntimeImpl` 创建 Workflow 时渲染以下字段：

- **Param**：支持自引用，最多迭代 10 次防止无限循环
- **Metadata.data**：引用 `Param` 中的值

```yaml
Param:
  env: "production"
  appName: "myapp"
  # 自引用：引用其他 Param
  namespace: "{{ Param.appName }}-{{ Param.env }}"     # 渲染为: myapp-production
  fullImage: "{{ Param.registry }}/{{ Param.appName }}" # 渲染为: myregistry.com/myapp

Metadate:
  type: in-config
  data:
    # 引用 Param
    K8sNamespace: "{{ Param.namespace }}"
```

**渲染规则：**

| 场景 | 行为 |
|------|------|
| Param 引用 Param | 支持自引用，迭代渲染直到稳定或达到 10 次 |
| Metadata 引用 Param | 使用 `{{ Param.xxx }}` 语法 |
| Metadata 自引用 | 不支持，Metadata 只能引用 Param |
| 引用未定义变量 | 模板表达式保持原样不变 |

### 2. 节点执行阶段（运行时）

每个步骤的 `run` 命令在执行时实时渲染，可以引用动态产生的 Metadata：

```yaml
Nodes:
  Build:
    executor: docker
    image: golang:1.21
    extract:
      type: codec-block
    steps:
      - name: build
        run: |
          go build -o app .
          echo '```flowx-yaml'
          echo 'version: "{{ Param.version }}"  # 版本号'
          echo 'binary: "app"  # 二进制文件名'
          echo '```'

  Deploy:
    executor: k8s
    steps:
      - name: deploy
        # 可以引用前面节点提取的 Metadata
        run: kubectl set image deployment/app app={{ Metadata.binary }}:{{ Metadata.version }}
```

## 模板语法参考

### 变量引用

```
{{ Param.key }}          # 引用 Param 中的值
{{ Metadata.key }}       # 引用 Metadata 中的值
{{ Param.nested.key }}   # 引用嵌套值（点分隔键自动展开）
```

### 条件表达式

```
{% if Param.env == "production" %}production{% else %}staging{% endif %}
```

### 过滤器

pongo2 内置 60+ 过滤器，常用：

| 过滤器 | 示例 | 说明 |
|--------|------|------|
| `default` | `{{ Param.value\|default:"fallback" }}` | 默认值 |
| `upper` | `{{ Param.name\|upper }}` | 转大写 |
| `lower` | `{{ Param.name\|lower }}` | 转小写 |
| `truncatechars` | `{{ Param.text\|truncatechars:50 }}` | 截断 |
| `join` | `{{ Param.list\|join:"," }}` | 连接列表 |
| `length` | `{{ Param.list\|length }}` | 长度 |
| `date` | `{{ Param.timestamp\|date:"Y-m-d" }}` | 日期格式化 |

### FlowX 自定义过滤器

| 过滤器 | 示例 | 说明 |
|--------|------|------|
| `toJson` | `{{ Param.data\|toJson }}` | 值转 JSON 字符串（map/slice 序列化，字符串原样返回） |
| `toYaml` | `{{ Param.data\|toYaml }}` | 值转 YAML 字符串 |
| `toBase64` | `{{ Param.text\|toBase64 }}` | 字符串 Base64 编码 |
| `fromBase64` | `{{ Param.encoded\|fromBase64 }}` | Base64 解码为字符串 |
| `urlencode` | `{{ Param.url\|urlencode }}` | URL 编码 |
| `urldecode` | `{{ Param.encoded\|urldecode }}` | URL 解码 |

### 自定义过滤器

```go
import "github.com/flosch/pongo2/v6"

pongo2.RegisterFilter("myFilter", func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
    return pongo2.AsValue(strings.ToUpper(in.String())), nil
})
```

## 构建渲染上下文

`buildRenderContext()` 方法将 Param 和 Metadata 合并为模板上下文：

```go
// 合并结果：
// 1. Param 值作为顶层键
// 2. Param 值同时以 "Param.xxx" 形式存在
// 3. Metadata 键展开（点分隔键转为嵌套对象）
// 4. Metadata 中的 JSON 字符串自动解析为对象/数组
//
// 例如 Param: {env: "prod"}, Metadata: {"Build.result": "ok", "Build.jsonData": '{"key": "val"}'}
// 最终上下文：
// {
//   "env": "prod",
//   "Param": {"env": "prod"},
//   "Build": {"result": "ok", "jsonData": {"key": "val"}}
// }
```

### 嵌套对象展开

Metadata 中包含点号（`.`）的键会被展开为嵌套对象：

```
"Node1.result" → {"Node1": {"result": "value"}}
"Node1.data.key" → {"Node1": {"data": {"key": "value"}}}
```

这使得模板中可以直接使用 `{{ Node1.result }}` 引用。

### JSON 字符串自动解析

Metadata 中的字符串值如果符合 JSON 格式（以 `{` `}` 或 `[` `]` 包裹），会自动解析为对象或数组：

```yaml
Nodes:
  Build:
    executor: local
    extract:
      type: codec-block
    steps:
      - name: build
        run: |
          echo '```flowx-yaml'
          echo 'forecasts: "[{"day":"周一","temp":"25"}]"  # 天气数据'
          echo '```'
  Notify:
    executor: local
    steps:
      - name: notify
        # forecasts 在上下文中是 []interface{} 而非字符串
        run: echo "{{ Build.forecasts[0].day }}"  # 输出：周一
```
