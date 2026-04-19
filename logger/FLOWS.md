# Logger 模块流程图

## Pusher 接口

```mermaid
flowchart TD
    A[Pusher] --> B[Push<br/>ctx, Entry<br/>推送单条日志]
    A --> C[PushBatch<br/>ctx, []Entry<br/>批量推送]
    A --> D[Close<br/>关闭连接<br/>刷新缓冲]
```

## Entry 日志条目

```mermaid
flowchart TD
    A[Entry] --> B[Pipeline<br/>流水线ID]
    A --> C[BuildID<br/>构建ID]
    A --> D[Node<br/>节点名称]
    A --> E[Step<br/>步骤名称]
    A --> F[Timestamp<br/>时间戳]
    A --> G[Level<br/>日志级别<br/>debug/info/warn/error]
    A --> H[Message<br/>日志消息]
    A --> I[Output<br/>命令输出<br/>stdout/stderr]
```

## 日志级别

```mermaid
flowchart LR
    A[Level] --> B[debug<br/>调试信息]
    A --> C[info<br/>普通信息]
    A --> D[warn<br/>警告信息]
    A --> E[error<br/>错误信息]

    B --> F[最详细]
    C --> G[一般]
    D --> H[重要]
    E --> I[最严重]
```

## ConsolePusher 实现

```mermaid
flowchart TD
    subgraph Push
    A[ConsolePusher.Push] --> B{日志级别}
    B -->|debug| C[fmt.Fprint<br/>stdout]
    B -->|info| D[fmt.Fprint<br/>stdout]
    B -->|warn| E[fmt.Fprint<br/>stderr]
    B -->|error| F[fmt.Fprint<br/>stderr]
    end

    subgraph Close
    G[ConsolePusher.Close] --> H[flush缓冲区<br/>return nil]
    end
```

## 日志输出格式

```mermaid
flowchart LR
    subgraph 输出示例
    A["[2024-01-19 10:30:45] [info] [pipeline-123] [node-1] [step-1] 正在执行命令"]
    end

    A --> B[时间戳]
    A --> C[级别]
    A --> D[PipelineID]
    A --> E[NodeID]
    A --> F[StepID]
    A --> G[消息内容]
```

## 日志推送流程

```mermaid
sequenceDiagram
    participant P as Pipeline
    participant E as Executor
    participant PP as Pusher
    participant C as Console

    E->>PP: Push(Entry{Level: info, Message: "building..."})
    PP->>C: fmt.Fprint stdout
    E->>PP: Push(Entry{Level: info, Output: "step 1"})
    PP->>C: fmt.Fprint stdout
    E->>PP: Push(Entry{Level: error, Message: "failed"})
    PP->>C: fmt.Fprint stderr
```

## 日志批量推送

```mermaid
flowchart TD
    A[PushBatch] --> B{entries非空?}
    B -->|否| C[return nil]
    B -->|是| D{遍历entries}
    D --> E[单个Push]
    E --> F{遍历完成?}
    F -->|否| D
    F -->|是| G[return nil]
```