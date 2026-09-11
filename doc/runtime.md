# 运行时管理

本文档介绍 Runtime 接口、同步/异步执行模式、快照恢复、配置导出和后台清理等功能。

## Runtime 接口

```go
type Runtime interface {
    Get(id string) (Workflow, error)                                          // 获取流水线
    Cancel(ctx context.Context, id string) error                              // 取消流水线
    RunAsync(ctx, id, configYAML string, listener Listener) (Workflow, error) // 异步执行
    RunSync(ctx, id, configYAML string, listener Listener) (Workflow, error)  // 同步执行
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
    LoadWorkflow(ctx, id, config, listener) (dag.Workflow, error)                      // 加载流水线（含快照恢复）但不运行
    Rerun(ctx context.Context, id string) error                                         // 重新运行可修改状态的流水线（已完成节点跳过）
    UpdateConfig(ctx context.Context, id string, newConfigYAML string) error              // 通过新配置更新流水线
    ModifyGraph(ctx context.Context, id string, modifications GraphModifications) error // 动态修改图
}
```

## 创建 Runtime

```go
rt := flowx.NewRuntime(context.Background())
```

`RuntimeImpl` 使用 `sync.RWMutex` 保证并发安全，内部维护一个 `map[string]Workflow` 管理所有流水线实例。

## 执行模式

### 同步执行 RunSync

在调用 goroutine 中阻塞执行，直到流水线完成。

```go
p, err := rt.RunSync(ctx, "workflow-001", configYAML, listener)
if err != nil {
    log.Fatal(err)
}
// p.Status() 此时已经是终态
fmt.Println("Workflow completed:", p.Status())
```

### 异步执行 RunAsync

在新的 goroutine 中执行，立即返回 Workflow 实例。

```go
p, err := rt.RunAsync(ctx, "workflow-001", configYAML, listener)
if err != nil {
    log.Fatal(err)
}

// 做其他事情...

// 等待完成
<-p.Done()
fmt.Println("Workflow completed:", p.Status())
```

**注意：** 异步执行的 Workflow 在完成后会自动从 Runtime 的内部 map 中移除。

### 重复 ID 处理

如果使用已存在的 ID 调用 `RunSync` 或 `RunAsync`，会返回错误：

```go
_, err := rt.RunSync(ctx, "workflow-001", configYAML, nil) // 成功
_, err = rt.RunSync(ctx, "workflow-001", configYAML, nil)   // 错误：重复 ID
```

## 执行流程

`RunSync` 和 `RunAsync` 的内部流程：

```
1. 解析 YAML 配置 → WorkflowConfig
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
6. 创建 Workflow 实例
   ├── 注入 Graph、Metadata、Provider、TemplateEngine
   └── 注册 Listener
      ↓
7. 执行 Workflow.Run()
```

## 流水线管理

### 获取流水线

```go
p, err := rt.Get("workflow-001")
if err != nil {
    // 流水线不存在
}
```

### 取消流水线

```go
err := rt.Cancel(ctx, "workflow-001")
// 取消正在运行的流水线
// 所有执行器收到取消信号
// 节点状态更新为 CANCELLED
```

### 移除流水线

```go
rt.Rm("workflow-001")
// 从 Runtime 中移除流水线记录
```

## 快照与恢复

### 导出配置

`ExportConfig` 将正在运行的流水线状态导出为 YAML 字符串，包含所有节点和步骤的运行时状态。

```go
yamlStr, err := rt.ExportConfig("workflow-001")
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
yamlStr, _ := rt.ExportConfig("workflow-001")

// 2. 稍后恢复
p, _ := rt.RunSync(ctx, "workflow-001-recovered", yamlStr, nil)
// 已完成的节点（状态为 SUCCESS/FAILED/CANCELLED）会被跳过
// 从中断处继续执行
```

### Snapshotter 接口

```go
type Snapshotter interface {
    TakeSnapshot(workflow Workflow, originalConfig *WorkflowConfig) (*WorkflowConfig, error)
    ToYAML(config *WorkflowConfig) (string, error)
    FromYAML(yamlStr string) (*WorkflowConfig, error)
}
```

`WorkflowSnapshotter` 实现深拷贝原始配置，然后将运行时状态注入到配置中：

1. 深拷贝原始 `WorkflowConfig`
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

清理逻辑：移除状态为终态（`SUCCESS`、`FAILED`、`ABORTED`）的 Workflow 记录。

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
err := rt.Pause(ctx, "workflow-001")
```

暂停后流水线状态变为 `PAUSED`，可以通过 `ExportConfig` 导出状态，或通过 `ModifyGraph` 修改图结构。

### 恢复流水线

`Resume` 恢复暂停或停止的流水线：

```go
err := rt.Resume(ctx, "workflow-001")
```

恢复后会重新计算 BFS 层级（图可能已被修改），从暂停时的层级继续执行。

## 配置更新

`UpdateConfig` 通过提供新的 YAML 配置来更新流水线。FlowX 自动计算新旧配置的差异并应用变更：

```go
newConfig := `
Version: "1.0"
Name: updated-workflow

Graph: |
  stateDiagram-v2
    [*] --> Build
    Build --> Deploy
    Deploy --> [*]

Nodes:
  Build:
    executor: local
    steps:
      - name: build
        run: echo "Building..."
  Deploy:
    executor: local
    steps:
      - name: deploy
        run: echo "Deploying..."
`

err := rt.UpdateConfig(ctx, "workflow-001", newConfig)
```

**规则：**
- 已执行的节点不能被移除或修改
- 新节点将在恢复后执行
- 边只能在两端节点都未执行时被移除

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

err := rt.ModifyGraph(ctx, "workflow-001", mods)
```

### 运行结束后追加节点并继续执行（ExportConfig 快照 + LoadWorkflow + Rerun）

流水线运行结束时通过 `ExportConfig` 导出包含节点运行时状态的快照 YAML 并自行持久化
（`RunAsync` 完成后实例即从 Runtime 删除，导出必须在 `WorkflowFinish` 事件回调内同步完成）。
之后（即使进程已重启）可从快照恢复并增量续跑：

```go
// 运行结束（WorkflowFinish 事件内）导出快照并保存
snapshotYAML, _ := rt.ExportConfig("workflow-001")

// 之后从快照恢复（不运行）；节点运行时状态随配置恢复，
// 流水线状态自动推导（FAILED > STOPPED > SUCCESS），处于可修改状态
workflow, _ := rt2.LoadWorkflow(ctx, "workflow-001", snapshotYAML, listener)

// 修改图（例如追加节点）
_ = rt2.UpdateConfig(ctx, "workflow-001", newConfigYAML)

// 继续运行：已终结状态的节点自动跳过，仅执行新增节点
_ = rt2.Rerun(ctx, "workflow-001")

// 不再需要时释放
rt2.Rm("workflow-001")
```

`Rerun` 要求流水线仍在 Runtime 中且处于可修改状态；
节点按运行时状态跳过（SUCCESS/FAILED/CANCELLED），即增量执行而非全量重跑。

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
8. 触发 `WorkflowGraphModified` 事件

## 并发安全

`RuntimeImpl` 使用 `sync.RWMutex` 保护内部 Workflow map：

- `Get`、`ExportConfig` 使用读锁
- `RunSync`、`RunAsync`、`Rm`、`Cancel` 使用写锁
- 多个 goroutine 可以安全地并发调用 Runtime 方法
