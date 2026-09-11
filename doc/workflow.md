# 流水线核心

本文档介绍 Workflowx 的核心流水线机制：DAG 图结构、统一遍历算法（支持有环/无环图）、循环执行、暂停恢复、动态图修改和事件系统。

## Workflow 接口

```go
type Workflow interface {
    Id() string                                          // 流水线唯一 ID（UUID）
    GetGraph() Graph                                     // 获取 DAG 图
    SetGraph(graph Graph)                                // 设置 DAG 图
    Status() string                                      // 当前状态
    SetMetadata(store MetadataStore)                     // 注入元数据存储
    Metadata() Metadata                                  // 获取当前元数据副本
    Listening(listener Listener)                         // 注册事件监听器
    Done() <-chan struct{}                               // 完成通知 channel
    Run(ctx context.Context) error                       // 执行流水线（阻塞）
    Notify()                                             // 触发通知
    Cancel()                                             // 取消流水线
    SetExecutorProvider(provider ExecutorProvider)        // 设置执行器工厂
    SetTemplateEngine(engine TemplateEngine)             // 设置模板引擎
    GetTemplateEngine() TemplateEngine                   // 获取模板引擎
    SetPusher(pusher logger.Pusher)                      // 设置日志推送器
    Pause() error                                        // 暂停流水线（等待当前层完成）
    Resume(ctx context.Context) error                    // 恢复暂停的流水线
    IsModifiable() bool                                  // 当前是否可修改图
}
```

## DAG 图结构

### Graph 接口

```go
type GraphReader interface {
    Nodes() map[string]Node                  // 获取所有节点
    Edges() []Edge                           // 获取所有边
    Traversal(ctx, evalCtx, fn) error        // BFS 拓扑遍历（支持有环/无环图）
    GetNode(nodeID string) (Node, bool)      // 根据ID查找节点
    GetEdge(srcID, destID string) (Edge, bool) // 根据源/目标ID查找边
    IncomingEdges(nodeID string) []Edge      // 返回指向指定节点的所有边
    OutgoingEdges(nodeID string) []Edge      // 返回从指定节点出发的所有边
}

type Graph interface {
    GraphReader
    AddVertex(node Node)                     // 添加节点
    AddEdge(edge Edge) error                 // 添加边（含环检测）
    RemoveVertex(nodeID string) error        // 删除节点及其关联边
    RemoveEdge(srcID, destID string) error   // 删除指定的边
    HasCycle() bool                          // 检查图中是否存在环
}
```

### DGAGraph 实现

`DGAGraph` 是线程安全的 DAG 图实现（使用 `sync.RWMutex`），同时支持有环图和无环图。

**核心行为：**

- **环检测**：每次 `AddEdge` 都会执行环检测
  - 无条件环 → 返回 `ErrHasCycle`（禁止死循环）
  - 条件环（边带有表达式）→ 标记为回边（back-edge），允许创建
- **统一 BFS 遍历**：`Traversal` 和 `TraversalSteps` 使用 forwardGraph（排除回边）计算层级，有环图和无环图走同一代码路径
- **并发执行**：同一层级内的节点通过 goroutine 并行执行，层级之间串行执行
- **条件边**：遍历时评估边的条件表达式，条件为 false 的边会被跳过
- **动态修改**：支持在暂停状态下删除/添加节点和边（`RemoveVertex`、`RemoveEdge`）

### 构建图

```go
// 创建节点
node1 := NewDGANode("Build", "")
node2 := NewDGANode("Test", "")
node3 := NewDGANode("Deploy", "")

// 创建图
graph := &DGAGraph{}
graph.AddVertex(node1)
graph.AddVertex(node2)
graph.AddVertex(node3)

// 添加边（自动环检测）
graph.AddEdge(NewDGAEdge(node1, node2))     // Build -> Test
graph.AddEdge(NewDGAEdge(node2, node3))     // Test -> Deploy
```

### 从配置构建

通常通过 `Runtime` 从 YAML 配置自动构建图，Graph 字段使用 Mermaid `stateDiagram-v2` 语法：

```yaml
Graph: |
  stateDiagram-v2
    [*] --> Build
    Build --> Test
    Test --> Deploy
    Deploy --> [*]
```

引擎使用 `mermaid-check` 解析器解析此语法，自动创建节点和边。

## 统一 BFS 遍历算法

`Traversal` 和 `TraversalSteps` 使用同一套 BFS 层级计算逻辑（`traversalStepsUnlocked`），有环图和无环图无需区分。

### 无环图遍历

1. 构建 forwardGraph（无环图等同于原始图）
2. 计算 forwardGraph 中所有节点的入度
3. 入度为 0 的节点作为起始节点
4. 并发执行所有入度为 0 的节点
5. 节点执行完成后，评估其所有出边
6. 条件为 true 或无条件的边：将目标节点入度减 1
7. 重复步骤 4-6，直到所有节点执行完毕

```
起始节点: [Build]           入度: Build=0, Test=1, Deploy=1
  ↓ Build 完成
层级 2:   [Test]            入度: Test=0, Deploy=1
  ↓ Test 完成
层级 3:   [Deploy]          入度: Deploy=0
```

### 有环图遍历

有环图通过 **forwardGraph + 回边评估** 实现循环执行：

1. **forwardGraph**：排除所有回边（条件边产生的环）后的邻接表
2. **层级计算**：在 forwardGraph 上执行 BFS，得到拓扑层级
3. **逐层执行**：按层级执行所有节点
4. **回边评估**：所有层级执行完毕后，评估回边条件
   - 条件为 true → 重置循环节点的运行时状态，从头开始下一轮迭代
   - 条件为 false → 循环结束
5. **迭代安全**：通过 `MaxLoopIterations`（默认 100）防止无限循环

```
图结构: A → B → C → D，回边 C → A（条件: iteration < 3）

迭代 0: A → B → C → D  → 评估回边 C→A: iteration(1) < 3 → true
迭代 1: A → B → C → D  → 评估回边 C→A: iteration(2) < 3 → true
迭代 2: A → B → C → D  → 评估回边 C→A: iteration(3) < 3 → false → 结束
```

### 配置示例

```yaml
MaxLoopIterations: 5          # 最大迭代次数（默认 100）

Graph: |
  stateDiagram-v2
    [*] --> A
    A --> B
    B --> C
    C --> A: {{ iteration < 3 }}    # 条件回边，形成可控循环
    C --> D
    D --> [*]
```

> `iteration` 是内置的迭代计数器变量，从 0 开始，每次循环 +1。详见 [条件边](edge.md) 中 `iteration` 说明。

## 流水线生命周期

### 状态流转

```
Init → Running → Success
                → Failed
                → Cancelled
                → Paused → Running (恢复)
                → Stopped → Running (恢复)
```

### 执行流程

```
Run() 被调用
  ├── 触发 WorkflowStart 事件
  ├── 创建求值上下文（EvaluationContext）
  ├── 逐层 BFS 执行（runLevelByLevel）
  │   ├── 循环开始：
  │   │   ├── 设置 iteration 到求值上下文
  │   │   ├── 计算 BFS 层级（使用 forwardGraph）
  │   │   └── 逐层执行：
  │   │       ├── 检查暂停信号 → 保存层级，等待恢复
  │   │       ├── 检查取消信号 → 返回 ctx.Err()
  │   │       ├── 并发执行当前层级所有节点
  │   │       └── 重新计算后续层级（条件边可获取最新 metadata）
  │   └── 循环检测：
  │       ├── 无回边 → 返回（无环图）
  │       ├── 评估回边条件 → true → 重置循环节点，继续迭代
  │       └── 评估回边条件 → false → 循环结束
  ├── 清理所有执行器
  ├── 触发 WorkflowFinish 事件
  └── 关闭 doneChan
```

### 节点跳过机制

节点和步骤在以下终端状态下会被跳过：

- `SUCCESS` - 已成功完成
- `FAILED` - 已失败
- `CANCELLED` - 已取消

这使得快照恢复成为可能：导出当前状态后重新加载配置，已完成的节点不会重复执行。

### 取消机制

调用 `Cancel()` 会取消流水线的 `context`，进而：

1. 所有正在运行的执行器收到取消信号
2. 执行器终止当前进程
3. 节点状态更新为 `CANCELLED`
4. 流水线状态更新为 `CANCELLED`

## 事件系统

### 事件类型

| 常量 | 值 | 触发时机 |
|------|------|---------|
| `EventWorkflowInit` | `WorkflowInit` | 流水线初始化 |
| `EventWorkflowStart` | `WorkflowStart` | 流水线开始执行 |
| `EventWorkflowFinish` | `WorkflowFinish` | 流水线执行完成 |
| `EventWorkflowCancelled` | `WorkflowCancelled` | 流水线被取消 |
| `EventWorkflowPaused` | `WorkflowPaused` | 流水线暂停 |
| `EventWorkflowResumed` | `WorkflowResumed` | 流水线恢复 |
| `EventWorkflowGraphModified` | `WorkflowGraphModified` | 图被动态修改 |
| `EventWorkflowExecutorPrepare` | `WorkflowExecutorPrepare` | 执行器准备中 |
| `EventWorkflowExecutorPrepareDone` | `WorkflowExecutorPrepareDone` | 执行器准备完成 |
| `EventWorkflowStatusUpdate` | `WorkflowStatusUpdate` | 流水线状态更新 |
| `EventWorkflowNodeStart` | `WorkflowNodeStart` | 节点开始执行 |
| `EventWorkflowNodeFinish` | `WorkflowNodeFinish` | 节点执行完成 |

### Listener 接口

```go
type Listener interface {
    Handle(p Workflow, event Event)  // 处理事件
    Events() []Event                  // 订阅的事件列表
}
```

### 使用示例

```go
// 创建监听器
listener := &flowx.DGAListener{
    Events: []flowx.Event{
        flowx.EventWorkflowNodeStart,
        flowx.EventWorkflowNodeFinish,
    },
    Handler: func(p flowx.Workflow, event flowx.Event) {
        fmt.Printf("Event: %s\n", event)
    },
}

p, _ := rt.RunSync(ctx, "workflow-001", configYAML, listener)
```

## 并发安全

- `DGAGraph` 使用 `sync.RWMutex` 保护节点和边的并发访问
- `WorkflowImpl` 使用 `sync.Once` 确保 `doneChan` 只关闭一次
- 遍历过程中使用 `sync.WaitGroup` 等待并行节点
- 执行器按名称缓存，同一 Workflow 中同名执行器共享实例

## 暂停与恢复

### 暂停流水线

调用 `Pause()` 后，流水线会在当前层级的所有节点执行完成后暂停：

```go
workflow, _ := rt.RunAsync(ctx, "workflow-001", config, nil)

// 暂停（等待当前层执行完毕）
err := workflow.Pause()
```

### 恢复流水线

调用 `Resume()` 恢复暂停的流水线：

```go
err := workflow.Resume(ctx)
```

### 暂停期间可修改图

暂停状态下可以安全地修改图结构（添加/删除节点和边），恢复后会使用修改后的图重新计算层级。

## 动态图修改

在流水线处于 `PAUSED`、`STOPPED`、`FAILED`、`CANCELLED` 或 `SUCCESS` 状态时，可以通过 `Runtime.ModifyGraph` 修改图结构：

```go
mods := flowx.GraphModifications{
    RemoveNodes: []string{"OldNode"},
    AddNodes: []flowx.NodeConfig{
        {
            Name:     "NewNode",
            Executor: "local",
            Steps: []flowx.Step{
                {Name: "step1", Run: "echo hello"},
            },
        },
    },
    AddEdges: []flowx.EdgeModification{
        {Source: "NewNode", Target: "ExistingNode"},
    },
}

err := rt.ModifyGraph(ctx, "workflow-001", mods)
```

修改操作是原子的：如果任何一步失败，会自动回滚到修改前的状态。

### 修改操作顺序

1. 删除边
2. 删除节点（自动删除关联边）
3. 添加新节点
4. 添加新边
5. 解析 Mermaid 图片段（如果有）
6. 校验图结构（允许条件回边，拒绝无条件环）
7. 更新存储的配置
8. 触发 `WorkflowGraphModified` 事件
