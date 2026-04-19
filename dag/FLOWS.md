# DAG 模块流程图

## 图结构

```mermaid
flowchart TD
    subgraph DGAGraph核心结构
    A[DGAGraph] --> B[nodes<br/>map[string]Node]
    A --> C[edges<br/>map[string]Edge]
    A --> D[graph<br/>map[string][]string<br/>邻接表]
    A --> E[edgeMap<br/>map[string]map[string]Edge<br/>源→目标→边]
    A --> F[sequence<br/>[]string]
    A --> G[hasCycle<br/>bool]
    A --> H[backEdges<br/>map[string]bool<br/>被接受的回边]
    A --> I[entryNodes<br/>map[string]bool<br/>入口节点]
    A --> J[exitNodes<br/>map[string]bool<br/>出口节点]
    end

    B --> K[Node接口]
    K --> L[Id: string]
    K --> M[Name: string]
    K --> N[GetExecutor: string]
    K --> O[GetSteps: []core.Step]
    K --> P[GetRuntimeStatus: *NodeRuntimeStatus]
    K --> Q[SetRuntimeStatus]
    K --> R[EnsureIds]
```

## 边类型

```mermaid
flowchart TD
    subgraph Edge类型
    A[DGAEdge] --> |无条件边| B[普通顺序执行]
    C[ConditionalEdge] --> |有表达式| D[条件判断执行]
    end

    subgraph ConditionalEdge
    D --> E[Expression: 模板表达式]
    D --> F[Evaluate: 求值判断]
    F --> |true| G[执行目标节点]
    F --> |false| H[跳过目标节点]
    end
```

## Pipeline.Run 完整执行流程

```mermaid
flowchart TD
    A[Pipeline.Run] --> B[创建context<br/>设置cancelFunc]
    B --> C[status = RUNNING]
    C --> D[defer cleanup<br/>关闭doneChan<br/>取消context<br/>清理executors]

    D --> E[NotifyEvent<br/>PipelineStart]
    E --> F[创建EvaluationContext]
    F --> G{有MetadataStore?}
    G -->|是| H[加载metadata<br/>到evalCtx]
    G -->|否| I[继续]

    H --> I
    I --> J{是DGAGraph?}
    J -->|是| K[runLevelByLevel]
    J -->|否| L[graph.Traversal<br/>兼容旧实现]

    K --> M{执行结果}
    M -->|成功| N[status = SUCCESS]
    M -->|失败| O[status = FAILED]
    N --> P[NotifyEvent<br/>PipelineFinish]
    O --> P
    L --> P
```

## runLevelByLevel 逐层执行（核心）

```mermaid
flowchart TD
    A[runLevelByLevel] --> B[iteration = 0]
    B --> C[maxIter = maxLoopIter]
    C --> D{循环}
    D --> E[evalCtx.WithIteration<br/>iteration]
    E --> F[TraversalSteps<br/>计算BFS层级]

    F --> G{levels空?}
    G -->|是| Z[return nil]
    G -->|否| H[levelIdx = startLevel]

    H --> I{levelIdx < len<br/>levels?}
    I -->|是| J[检查pauseChan]
    J --> K{收到暂停信号?}
    K -->|是| L[保存currentLevel<br/>status = PAUSED]
    L --> M[NotifyEvent<br/>PipelinePaused]
    M --> N[等待resumeChan]

    N --> O[收到恢复信号]
    O --> P[status = RUNNING<br/>重置pause/resumeChan]
    P --> Q[重新计算层级]
    Q --> R{levelIdx >= len?}
    R -->|是| Z
    R -->|否| I

    K -->|否| S[检查ctx.Done]
    S --> T{context取消?}
    T -->|是| U[status = CANCELLED<br/>return ctx.Err]
    T -->|否| V[执行当前层]

    V --> W[并发执行<br/>当前层所有节点]
    W --> X[wg.Wait<br/>等待完成]
    X --> Y{有错误?}
    Y -->|是| AA[return firstErr]
    Y -->|否| AB[levelIdx++<br/>重新计算后续层级]

    AB --> I

    I -->|否| AC[遍历完所有层]
    AC --> AD{有回边?}
    AD -->|无| Z
    AD -->|有| AE[评估回边条件]

    AE --> AF[回边条件满足?]
    AF -->|否| Z
    AF -->|是| AG[iteration++]
    AG --> AH{iteration >=<br/>maxIter?}
    AH -->|是| AI[return err<br/>超过最大迭代]
    AH -->|否| AJ[resetLoopNodes<br/>重置循环节点状态]
    AJ --> AK[startLevel = 0]
    AK --> D
```

## executeNodeWithLifecycle 节点生命周期

```mermaid
flowchart TD
    A[executeNodeWithLifecycle] --> B{ctx.Done?}
    B -->|是| C[return ctx.Err]
    B -->|否| D{shouldSkipNode?}
    D -->|是| E[打印跳过日志<br/>NotifyEvent<br/>PipelineNodeFinish<br/>return nil]

    D -->|否| F[NotifyEvent<br/>PipelineNodeStart]
    F --> G[打印执行日志]

    G --> H{有executor?}
    H -->|无| I[打印警告<br/>NotifyEvent<br/>PipelineNodeFinish<br/>return nil]

    H -->|有| J[getOrCreateExecutor<br/>获取或创建executor]
    J --> K[executeNode]
    K --> L{执行结果}
    L -->|成功| M[NotifyEvent<br/>PipelineNodeFinish<br/>return nil]
    L -->|失败| N[return err]
```

## executeNode 单节点执行

```mermaid
flowchart TD
    A[executeNode] --> B{steps空?}
    B -->|是| Z[return nil]
    B -->|否| C[initializeNode<br/>RuntimeStatus]

    C --> D[创建channels<br/>commandChan<br/>resultChan<br/>inputChan]
    D --> E[启动exec.Transfer<br/>goroutine]
    E --> F[保存inputChan<br/>到RuntimeStatus]

    F --> G[sendCommands<br/>goroutine<br/>发送所有步骤命令]
    G --> H[waitForResults<br/>等待处理所有结果]

    H --> I{lastErr?}
    I -->|有错误| J[跳过提取]
    I -->|无错误| K[extractOutput<br/>提取metadata]

    J --> L[updateNode<br/>FinalStatus]
    K --> L
    L --> M[close inputChan]
    M --> N[return lastErr]
```

## sendCommands 发送命令

```mermaid
flowchart TD
    A[sendCommands] --> B[defer close<br/>commandChan]
    B --> C{遍历steps}
    C --> D[检查ctx.Done]
    D --> E{shouldSkipStep?}
    E -->|是| F[跳过该步骤]
    F --> C
    E -->|否| G[renderString<br/>渲染命令模板]

    G --> H{渲染成功?}
    H -->|失败| I[使用原始命令]
    I --> J[发送到commandChan<br/>CommandWrapper]
    H -->|成功| J

    J --> C
    C -->|遍历完成| K[return]
```

## waitForResults 等待结果

```mermaid
flowchart TD
    A[waitForResults] --> B[计算期望结果数<br/>跳过已完成的步骤]
    B --> C{resultCount <<br/>expectedResults?}
    C -->|是| D[select]
    C -->|否| Z[return<br/>lastErr, output]

    D --> E{ctx.Done?}
    E -->|是| F[handleCancellation<br/>return ctx.Err]
    E -->|否| G{resultChan关闭?}
    G -->|是| Z
    G -->|否| H[handleResult<br/>处理结果]

    H --> I{result类型}
    I -->|error| J[记录错误]
    I -->|StepResult| K[更新步骤状态]
    I -->|[]byte| L[实时输出<br/>pusher.Push]
    I -->|InputRequestEvent| M[handleInputRequest]
    I -->|InputReadyEvent| N[忽略]

    J --> O[resultCount++]
    K --> O
    L --> O
    M --> O
    O --> C
```

## handleInputRequest 输入请求处理

```mermaid
flowchart TD
    A[handleInputRequest] --> B{event有效?}
    B -->|否| Z[return]
    B -->|是| C[获取RuntimeStatus]
    C --> D[status = PAUSED]
    D --> E[设置InputRequest<br/>StepName<br/>Prompt<br/>Type]
    E --> F[node.Set<br/>RuntimeStatus]

    F --> G[NotifyEvent<br/>PipelinePaused]
    G --> H[等待外部输入<br/>InputChan接收数据]
    H --> I[继续处理]
```

## 循环图执行流程

```mermaid
flowchart TD
    A[循环图执行] --> B[遍历完所有层级]
    B --> C[BackEdges获取回边]
    C --> D{有回边?}
    D -->|无| Z[return<br/>无环图完成]
    D -->|有| E[遍历每条回边]

    E --> F[Evaluate条件]
    F --> G{条件为true?}
    G -->|否| H[继续检查下一回边]
    G -->|是| I[shouldContinue = true]
    I --> J[LoopNodeSet<br/>获取循环节点集合]

    J --> K[合并所有活跃<br/>循环节点]
    K --> L{所有回边检查完}
    L -->|否| E
    L -->|是| M{shouldContinue?}

    M -->|false| Z
    M -->|true| N[iteration++]
    N --> O{超过maxIter?}
    O -->|是| P[return err<br/>循环超限]
    O -->|否| Q[resetLoopNodes<br/>重置节点状态]

    Q --> R[清理metadata<br/>设置startLevel=0]
    R --> S[重新开始遍历]
    S --> B
```

## resetLoopNodes 重置循环节点

```mermaid
flowchart TD
    A[resetLoopNodes] --> B{遍历loopNodes}
    B --> C[GetNode获取节点]
    C --> D[SetRuntimeStatus<br/>nil<br/>清除跳过标记]
    D --> E{节点有metadata?}
    E -->|是| F[删除以nodeID.前缀<br/>的metadata条目]
    E -->|否| G[继续下一节点]
    F --> B
    G --> B
    B -->|遍历完成| H[return]
```

## 图构建流程

```mermaid
flowchart TD
    A[buildGraph] --> B[NewDGAGraph]
    B --> C{遍历config.Nodes}
    C --> D[创建DGANode<br/>包含executor<br/>steps等]
    D --> E[AddVertex<br/>添加到图]
    E --> C

    C -->|完成| F{config.Graph<br/>有定义?}
    F -->|否| Z[return graph]
    F -->|是| G[parseGraphEdges<br/>解析图边关系]

    G --> H[使用mermaid-check<br/>解析器]
    H --> I[遍历Statements]
    I --> J{是Transition?}
    J -->|是| K[处理入口节点<br/>[*] --> X]
    K --> L[处理出口节点<br/>X --> [*]]
    L --> M[提取条件表达式]
    M --> N{有条件?}
    N -->|是| O[NewConditionalEdge]
    N -->|否| P[NewDGAEdge]
    O --> Q[AddEdge]
    P --> Q
    Q --> I
```

## 事件系统

```mermaid
flowchart LR
    subgraph 事件类型
    A[PipelineInit]
    B[PipelineStart]
    C[PipelineFinish]
    D[PipelineNodeStart]
    E[PipelineNodeFinish]
    F[PipelinePaused]
    G[PipelineResumed]
    H[PipelineGraphModified]
    I[PipelineCancelled]
    J[PipelineStatusUpdate]
    end

    A --> K[Listener.Handle]
    B --> K
    C --> K
    D --> K
    E --> K
    F --> K
    G --> K
    H --> K
    I --> K
    J --> K
```

## 状态流转

```mermaid
stateDiagram-v2
    [*] --> PENDING
    PENDING --> RUNNING: Pipeline.Run

    RUNNING --> PAUSED: Pause请求<br/>输入请求
    PAUSED --> RUNNING: Resume
    PAUSED --> CANCELLED: Cancel

    RUNNING --> SUCCESS: 所有节点完成
    RUNNING --> FAILED: 节点执行失败
    RUNNING --> CANCELLED: Cancel

    SUCCESS --> STOPPED: 清理完成
    FAILED --> STOPPED: 清理完成
    CANCELLED --> STOPPED: 清理完成

    STOPPED --> [*]

    note right of PAUSED: 等待用户输入<br/>或外部恢复信号
```

## 暂停恢复机制

```mermaid
sequenceDiagram
    participant P as Pipeline
    participant T as runLevelByLevel
    participant PC as pauseChan
    participant RC as resumeChan
    participant L as Listener

    T->>T: 执行当前层
    T->>PC: select检查pauseChan
    P->>P: Pause()被调用
    P->>PC: close(pauseChan)
    T->>T: 检测到暂停信号
    T->>T: 保存currentLevel
    T->>T: status = PAUSED
    T->>L: NotifyEvent(PipelinePaused)
    T->>T: 等待resumeChan

    Note over T: 外部处理输入/恢复
    P->>P: Resume()被调用
    P->>RC: close(resumeChan)
    T->>T: 检测到恢复信号
    T->>T: status = RUNNING
    T->>T: 重置pause/resumeChan
    T->>L: NotifyEvent(PipelineResumed)
    T->>T: 重新计算层级
    T->>T: 继续执行
```