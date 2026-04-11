# 流水线核心

本文档介绍 Pipelinex 的核心流水线机制：DAG 图结构、遍历算法、生命周期管理和事件系统。

## Pipeline 接口

```go
type Pipeline interface {
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
}
```

## DAG 图结构

### Graph 接口

```go
type GraphReader interface {
    Nodes() map[string]Node                  // 获取所有节点
    Edges() []Edge                           // 获取所有边
    Traversal(ctx, evalCtx, fn) error        // BFS 拓扑遍历
}

type Graph interface {
    GraphReader
    AddVertex(node Node)                     // 添加节点
    AddEdge(edge Edge) error                 // 添加边（含环检测）
}
```

### DGAGraph 实现

`DGAGraph` 是线程安全的 DAG 图实现（使用 `sync.RWMutex`）。

**核心行为：**

- **环检测**：每次 `AddEdge` 都会执行环检测，如果检测到环路则返回 `ErrHasCycle`
- **BFS 拓扑遍历**：`Traversal` 方法执行广度优先的拓扑排序遍历
- **并发执行**：同一层级（入度为 0）的节点通过 goroutine 并行执行
- **条件边**：遍历时评估边的条件表达式，条件为 false 的边会被跳过（目标节点入度不减）

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

## BFS 遍历算法

遍历过程：

1. 计算所有节点的入度
2. 入度为 0 的节点作为起始节点
3. 并发执行所有入度为 0 的节点
4. 节点执行完成后，评估其所有出边
5. 条件为 true 或无条件的边：将目标节点入度减 1
6. 重复步骤 3-5，直到所有节点执行完毕
7. 使用 `sync.WaitGroup` 等待所有并发节点完成

```
起始节点: [Build]           入度: Build=0, Test=1, Deploy=1
  ↓ Build 完成
层级 2:   [Test]            入度: Test=0, Deploy=1
  ↓ Test 完成
层级 3:   [Deploy]          入度: Deploy=0
```

## 流水线生命周期

### 状态流转

```
Init → Running → Success
                → Failed
                → Cancelled (ABORTED)
```

### 执行流程

```
Run() 被调用
  ├── 触发 PipelineStart 事件
  ├── 遍历 DAG 图
  │   ├── 对每个节点：
  │   │   ├── 检查状态（跳过已完成/失败/取消的节点）
  │   │   ├── 触发 PipelineExecutorPrepare 事件
  │   │   ├── 通过 Provider 获取执行器
  │   │   ├── 触发 PipelineNodeStart 事件
  │   │   ├── 依次执行步骤（渲染命令模板）
  │   │   ├── 收集输出，执行输出提取
  │   │   ├── 触发 PipelineNodeFinish 事件
  │   │   └── 更新节点状态
  │   └── 等待所有并行节点完成
  ├── 清理所有执行器
  ├── 触发 PipelineFinish 事件
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
4. 流水线状态更新为 `ABORTED`

## 事件系统

### 事件类型

| 常量 | 值 | 触发时机 |
|------|------|---------|
| `EventPipelineInit` | `PipelineInit` | 流水线初始化 |
| `EventPipelineStart` | `PipelineStart` | 流水线开始执行 |
| `EventPipelineFinish` | `PipelineFinish` | 流水线执行完成 |
| `EventPipelineCancelled` | `PipelineCancelled` | 流水线被取消 |
| `EventPipelineExecutorPrepare` | `PipelineExecutorPrepare` | 执行器准备中 |
| `EventPipelineExecutorPrepareDone` | `PipelineExecutorPrepareDone` | 执行器准备完成 |
| `EventPipelineNodeStart` | `PipelineNodeStart` | 节点开始执行 |
| `EventPipelineNodeFinish` | `PipelineNodeFinish` | 节点执行完成 |

### Listener 接口

```go
type Listener interface {
    Handle(p Pipeline, event Event)  // 处理事件
    Events() []Event                  // 订阅的事件列表
}
```

### 使用示例

```go
// 创建监听器
listener := &pipelinex.DGAListener{
    Events: []pipelinex.Event{
        pipelinex.EventPipelineNodeStart,
        pipelinex.EventPipelineNodeFinish,
    },
    Handler: func(p pipelinex.Pipeline, event pipelinex.Event) {
        fmt.Printf("Event: %s\n", event)
    },
}

p, _ := rt.RunSync(ctx, "pipeline-001", configYAML, listener)
```

## 并发安全

- `DGAGraph` 使用 `sync.RWMutex` 保护节点和边的并发访问
- `PipelineImpl` 使用 `sync.Once` 确保 `doneChan` 只关闭一次
- 遍历过程中使用 `sync.WaitGroup` 等待并行节点
- 执行器按名称缓存，同一 Pipeline 中同名执行器共享实例
