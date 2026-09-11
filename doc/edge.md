# 条件边

本文档介绍 DAG 图中条件边的概念、表达式求值机制以及条件分支控制。

## Edge 接口

```go
type Edge interface {
    Source() Node                                    // 源节点
    Target() Node                                    // 目标节点
    Expression() string                              // 条件表达式（空字符串表示无条件）
    Evaluate(ctx EvaluationContext) (bool, error)    // 评估条件
    ID() string                                      // 唯一标识 "source->target"
}
```

## 边的类型

### 无条件边

```go
edge := NewDGAEdge(sourceNode, targetNode)
// Expression() 返回 ""
// Evaluate() 始终返回 true
```

### 条件边

```go
// 使用默认模板引擎
edge := NewConditionalEdge(sourceNode, targetNode, "Param.env == \"production\"")

// 指定模板引擎
edge := NewConditionalEdgeWithEngine(sourceNode, targetNode, expression, engine)
```

## EvaluationContext（求值上下文）

`EvaluationContext` 为表达式提供求值所需的数据。

```go
type EvaluationContext interface {
    Get(key string) (any, bool)                      // 查找值
    All() map[string]any                             // 获取完整上下文
    WithNode(node Node) EvaluationContext            // 附加节点信息（返回新实例）
    WithWorkflow(workflow Workflow) EvaluationContext // 附加流水线信息（返回新实例）
    WithParams(params map[string]any) EvaluationContext // 附加参数（返回新实例）
    WithIteration(iteration int) EvaluationContext   // 附加迭代计数器（返回新实例）
    Iteration() int                                  // 获取当前迭代计数器值
}
```

### 上下文数据来源

`All()` 方法合并以下数据：

| 来源 | 键 | 说明 |
|------|-----|------|
| 节点 | `nodeId`, `nodeStatus` | 当前遍历的节点信息 |
| 流水线 | `workflowId`, `workflowStatus` | 流水线实例信息 |
| 参数 | `Param.xxx` | 配置中定义的全局参数 |
| 参数 | 直接键名 | Param 的键同时作为顶层键暴露 |
| 元数据 | 展开的嵌套结构 | Metadata 中的数据，点分隔键展开为嵌套对象 |
| 迭代计数器 | `iteration` | 当前循环迭代次数（从 0 开始），用于回边条件表达式 |

### 布尔转换

上下文中所有布尔值（`true`/`false`）会被递归转换为字符串 `"true"`/`"false"`，以确保模板表达式兼容性。

### 不可变性

`WithNode`、`WithWorkflow`、`WithParams`、`WithIteration` 返回新的实例，不修改原始上下文。

```go
ctx := NewEvaluationContext()
ctxWithNode := ctx.WithNode(node)    // 新实例
ctxWithParams := ctx.WithParams(params) // 新实例，原始 ctx 不变
ctxWithIter := ctx.WithIteration(3)  // 新实例，iteration=3
```

## 条件表达式语法

条件表达式使用 [pongo2](https://github.com/flosch/pongo2) 模板语法（类似 Django 模板语言）。

### 基本比较

```yaml
Graph: |
  stateDiagram-v2
    [*] --> Check
    Check --> Deploy : {{ Param.env == "production" }}
    Check --> Skip   : {{ Param.env != "production" }}
```

### 逻辑运算

```yaml
# AND 条件
Test --> Deploy : {{ Param.testsPassed and Param.coverage > 80 }}

# OR 条件
Check --> Deploy : {{ Param.env == "production" or Param.env == "staging" }}

# NOT 条件
Check --> Skip : {{ not Param.skipTests }}
```

### 比较运算符

| 运算符 | 说明 |
|--------|------|
| `==` | 等于 |
| `!=` | 不等于 |
| `>` | 大于 |
| `<` | 小于 |
| `>=` | 大于等于 |
| `<=` | 小于等于 |
| `in` | 包含于 |
| `not in` | 不包含于 |

### 字符串操作

```yaml
Check --> Deploy : {{ Param.env == "production" }}
Check --> Staging : {{ Param.branch.startswith("release/") }}
```

### 引用元数据

```yaml
Build --> Deploy : {{ Metadata.buildStatus == "success" }}
```

## 在配置中使用

### Mermaid 图中的条件边

在 `Graph` 字段中，边的标签如果包含 `{{` 或 `{%`，并且能通过 pongo2 语法校验，会被自动识别为条件边：

```yaml
Graph: |
  stateDiagram-v2
    [*] --> Validate
    Validate --> Build     : {{ Param.testsPassed }}
    Validate --> Skip      : {{ not Param.testsPassed }}
    Build --> Deploy       : {{ Param.env == "production" }}
    Build --> Staging      : {{ Param.env == "staging" }}
    Deploy --> [*]
    Staging --> [*]
    Skip --> [*]
```

### 条件边的解析

引擎解析 Mermaid 图时：

1. 读取边的标签文本
2. 检测是否包含 `{{` 或 `{%`
3. 使用模板引擎校验语法有效性
4. 有效则创建 `ConditionalEdge`，无效则创建无条件边

## 遍历时的条件评估

在 BFS 遍历过程中：

1. 节点执行完成后，评估其所有出边
2. 对每条边调用 `Evaluate(evaluationContext)`
3. 条件为 **true** 或边无条件的：目标节点入度减 1
4. 条件为 **false** 的：目标节点入度不变（该分支被剪枝）
5. 如果目标节点入度减为 0，加入下一轮执行队列

```
     Check
    /      \
  true    false
  /          \
Deploy      (被剪枝)
```

## 完整示例

```yaml
Version: "1.0"
Name: conditional-workflow

Param:
  env: "production"
  testsPassed: true
  coverage: 85

Executors:
  local:
    type: local

Graph: |
  stateDiagram-v2
    [*] --> Validate
    Validate --> Build   : {{ Param.testsPassed and Param.coverage > 80 }}
    Validate --> Fail    : {{ not Param.testsPassed }}
    Build --> Deploy     : {{ Param.env == "production" }}
    Build --> Staging    : {{ Param.env == "staging" }}
    Deploy --> [*]
    Staging --> [*]
    Fail --> [*]

Nodes:
  Validate:
    executor: local
    steps:
      - name: check
        run: echo "Validating..."
  Build:
    executor: local
    steps:
      - name: build
        run: echo "Building..."
  Deploy:
    executor: local
    steps:
      - name: deploy
        run: echo "Deploying to production"
  Staging:
    executor: local
    steps:
      - name: deploy
        run: echo "Deploying to staging"
  Fail:
    executor: local
    steps:
      - name: fail
        run: echo "Validation failed"
```

此配置中，当 `Param.env == "production"` 且 `Param.testsPassed == true` 且 `Param.coverage > 80` 时，执行路径为：

```
Validate → Build → Deploy
```

## 回边（Back-Edge）与循环图

当一条条件边在图中形成环路时，它会被标记为**回边**（back-edge），允许创建可控的循环执行。

### 工作原理

1. **添加边时**：如果条件边产生了环，引擎将其标记为回边并接受（无条件环则拒绝）
2. **遍历时**：回边被排除在 forwardGraph 之外，不参与 BFS 层级计算
3. **层级执行完毕后**：评估回边的条件表达式
   - 条件为 true → 重置循环节点的运行时状态，从头开始下一轮迭代
   - 条件为 false → 循环结束

### iteration 变量

回边条件表达式中可以使用 `iteration` 变量，它表示当前的迭代计数（从 0 开始）：

```yaml
Graph: |
  stateDiagram-v2
    [*] --> A
    A --> B
    B --> C
    C --> A: {{ iteration < 3 }}    # iteration=0,1,2 时继续，iteration=3 时停止
    C --> D
    D --> [*]
```

执行过程：

| 迭代轮次 | 执行的节点 | 评估 C→A 时 iteration 值 | `iteration < 3` | 结果 |
|----------|-----------|------------------------|-----------------|------|
| 0 | A→B→C→D | 1 | true | 继续循环 |
| 1 | A→B→C→D | 2 | true | 继续循环 |
| 2 | A→B→C→D | 3 | false | 循环结束 |

> 注意：评估回边时使用的是 `iteration + 1`（预判下一次迭代），因此 `iteration < 3` 实际执行 3 轮（iteration 0、1、2）。

### 循环安全

- 通过 `MaxLoopIterations` 配置（默认 100）防止无限循环
- 超过最大迭代次数会返回错误
- 循环节点的运行时状态在每次迭代前会被重置（包括 metadata 中该节点的前缀数据）
