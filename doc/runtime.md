# 运行时管理

本文档介绍 Runtime 接口、同步/异步执行模式、快照恢复、配置导出和后台清理等功能。

## Runtime 接口

```go
type Runtime interface {
    Get(id string) (Pipeline, error)                                          // 获取流水线
    Cancel(ctx context.Context, id string) error                              // 取消流水线
    RunAsync(ctx, id, configYAML string, listener Listener) (Pipeline, error) // 异步执行
    RunSync(ctx, id, configYAML string, listener Listener) (Pipeline, error)  // 同步执行
    Rm(id string)                                                             // 移除流水线
    Done() chan struct{}                                                      // 运行时关闭通知
    Notify(data any) error                                                    // 发送通知
    Ctx() context.Context                                                     // 获取上下文
    StopBackground()                                                          // 停止后台清理
    StartBackground()                                                         // 启动后台清理
    SetPusher(pusher logger.Pusher)                                           // 设置日志推送
    SetTemplateEngine(engine TemplateEngine)                                  // 设置模板引擎
    GetTemplateEngine() TemplateEngine                                        // 获取模板引擎
    ExportConfig(id string) (string, error)                                   // 导出运行时配置
    Pause(ctx context.Context, id string) error                               // 暂停流水线
    Resume(ctx context.Context, id string) error                              // 恢复流水线
    ModifyGraph(ctx context.Context, id string, modifications GraphModifications) error // 动态修改图
}
```

## 创建 Runtime

```go
rt := flowx.NewRuntime(context.Background())
```

`RuntimeImpl` 使用 `sync.RWMutex` 保证并发安全，内部维护一个 `map[string]Pipeline` 管理所有流水线实例。

## 执行模式

### 同步执行 RunSync

在调用 goroutine 中阻塞执行，直到流水线完成。

```go
p, err := rt.RunSync(ctx, "pipeline-001", configYAML, listener)
if err != nil {
    log.Fatal(err)
}
// p.Status() 此时已经是终态
fmt.Println("Pipeline completed:", p.Status())
```

### 异步执行 RunAsync

在新的 goroutine 中执行，立即返回 Pipeline 实例。

```go
p, err := rt.RunAsync(ctx, "pipeline-001", configYAML, listener)
if err != nil {
    log.Fatal(err)
}

// 做其他事情...

// 等待完成
<-p.Done()
fmt.Println("Pipeline completed:", p.Status())
```

**注意：** 异步执行的 Pipeline 在完成后会自动从 Runtime 的内部 map 中移除。

### 重复 ID 处理

如果使用已存在的 ID 调用 `RunSync` 或 `RunAsync`，会返回错误：

```go
_, err := rt.RunSync(ctx, "pipeline-001", configYAML, nil) // 成功
_, err = rt.RunSync(ctx, "pipeline-001", configYAML, nil)   // 错误：重复 ID
```

## 执行流程

`RunSync` 和 `RunAsync` 的内部流程：

```
1. 解析 YAML 配置 → PipelineConfig
      ↓
2. 渲染配置
   ├── 渲染 Param（自引用，最多 10 次迭代）
   └── 渲染 Metadata.data（引用 Param）
      ↓
3. 构建 DAG 图
   ├── 解析 Mermaid stateDiagram-v2
   ├── 创建节点和边
   └── 条件边识别（包含模板表达式的标签）
      ↓
4. 初始化元数据存储（MetadataStoreFactory）
      ↓
5. 创建执行器提供者（Provider）
   └── 注册配置中的 Executors
      ↓
6. 创建 Pipeline 实例
   ├── 注入 Graph、Metadata、Provider、TemplateEngine
   └── 注册 Listener
      ↓
7. 执行 Pipeline.Run()
```

## 流水线管理

### 获取流水线

```go
p, err := rt.Get("pipeline-001")
if err != nil {
    // 流水线不存在
}
```

### 取消流水线

```go
err := rt.Cancel(ctx, "pipeline-001")
// 取消正在运行的流水线
// 所有执行器收到取消信号
// 节点状态更新为 CANCELLED
```

### 移除流水线

```go
rt.Rm("pipeline-001")
// 从 Runtime 中移除流水线记录
```

## 快照与恢复

### 导出配置

`ExportConfig` 将正在运行的流水线状态导出为 YAML 字符串，包含所有节点和步骤的运行时状态。

```go
yamlStr, err := rt.ExportConfig("pipeline-001")
if err != nil {
    log.Fatal(err)
}
// yamlStr 包含完整的配置和运行时状态
```

### 导出内容

导出的 YAML 包含：

- 原始配置（Version、Name、Param、Executors、Graph、Nodes 等）
- 每个节点的 `NodeRuntimeStatus`（状态、时间、步骤状态、执行器信息）
- 每个步骤的 `StepRuntimeStatus`（状态、时间、输出、错误）

### 恢复执行

使用导出的配置作为输入，已完成的节点和步骤会被自动跳过：

```go
// 1. 导出当前状态
yamlStr, _ := rt.ExportConfig("pipeline-001")

// 2. 稍后恢复
p, _ := rt.RunSync(ctx, "pipeline-001-recovered", yamlStr, nil)
// 已完成的节点（状态为 SUCCESS/FAILED/CANCELLED）会被跳过
// 从中断处继续执行
```

### Snapshotter 接口

```go
type Snapshotter interface {
    TakeSnapshot(pipeline Pipeline, originalConfig *PipelineConfig) (*PipelineConfig, error)
    ToYAML(config *PipelineConfig) (string, error)
    FromYAML(yamlStr string) (*PipelineConfig, error)
}
```

`PipelineSnapshotter` 实现深拷贝原始配置，然后将运行时状态注入到配置中：

1. 深拷贝原始 `PipelineConfig`
2. 遍历所有图节点
3. 注入 `NodeRuntimeStatus` 到对应 `NodeConfig.Runtime`
4. 同步步骤 ID

## 后台清理

Runtime 提供后台 goroutine 定期清理已完成的流水线。

```go
// 启动后台清理（每 30 秒执行一次）
rt.StartBackground()

// 停止后台清理
rt.StopBackground()
```

清理逻辑：移除状态为终态（`SUCCESS`、`FAILED`、`ABORTED`）的 Pipeline 记录。

## 日志推送

Runtime 支持设置日志推送器（Pusher），将执行日志推送到外部系统。

```go
// 使用控制台推送器
pusher := logger.NewConsolePusher()
rt.SetPusher(pusher)
```

更多日志功能参见 [Logger 接口](../logger/logger.go)。

## 通知机制

```go
// 发送字符串通知
rt.Notify("deployment completed")

// 发送结构化数据
rt.Notify(map[string]any{
    "event": "build_complete",
    "image": "myapp:latest",
})

// 发送数字
rt.Notify(42)
```

## 暂停与恢复

### 暂停流水线

`Pause` 让流水线在当前 BFS 层级执行完成后暂停：

```go
err := rt.Pause(ctx, "pipeline-001")
```

暂停后流水线状态变为 `PAUSED`，可以通过 `ExportConfig` 导出状态，或通过 `ModifyGraph` 修改图结构。

### 恢复流水线

`Resume` 恢复暂停或停止的流水线：

```go
err := rt.Resume(ctx, "pipeline-001")
```

恢复后会重新计算 BFS 层级（图可能已被修改），从暂停时的层级继续执行。

## 动态图修改

`ModifyGraph` 在流水线处于可修改状态时（`PAUSED`、`STOPPED`、`FAILED`、`CANCELLED`、`SUCCESS`）执行图结构变更：

```go
mods := flowx.GraphModifications{
    RemoveNodes: []string{"OldNode"},
    RemoveEdges: []flowx.EdgeID{{Source: "A", Target: "B"}},
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
        {Source: "X", Target: "Y", Expression: "{{ env == 'prod' }}"},
    },
    AddGraph: "stateDiagram-v2\n  D --> E: {{ condition }}",
}

err := rt.ModifyGraph(ctx, "pipeline-001", mods)
```

### GraphModifications 结构

```go
type GraphModifications struct {
    RemoveNodes []string           // 要删除的节点 ID 列表
    RemoveEdges []EdgeID           // 要删除的边列表
    AddNodes    []NodeConfig       // 要添加的节点配置
    AddEdges    []EdgeModification // 要添加的边
    AddGraph    string             // Mermaid 图片段（解析后添加节点和边）
}
```

### 操作顺序与原子性

修改按以下顺序执行，任何步骤失败都会自动回滚：

1. 删除边
2. 删除节点（自动删除关联边）
3. 添加新节点
4. 添加新边
5. 解析 Mermaid 图片段
6. 校验图结构（允许条件回边，拒绝无条件环）
7. 更新存储的配置
8. 触发 `PipelineGraphModified` 事件

## 并发安全

`RuntimeImpl` 使用 `sync.RWMutex` 保护内部 Pipeline map：

- `Get`、`ExportConfig` 使用读锁
- `RunSync`、`RunAsync`、`Rm`、`Cancel` 使用写锁
- 多个 goroutine 可以安全地并发调用 Runtime 方法
