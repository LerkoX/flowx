# Runtime 模块流程图

## Runtime 接口

```mermaid
flowchart TD
    subgraph Runtime接口
    A[Get] --> |id| B[返回Pipeline]
    A --> C[RunAsync] --> |ctx,id,config,listener| D[异步执行]
    A --> E[RunSync] --> |ctx,id,config,listener| F[同步执行]
    A --> G[Cancel] --> |ctx,id| H[取消流水线]
    A --> I[Pause] --> |ctx,id| J[暂停流水线]
    A --> K[Resume] --> |ctx,id| L[恢复流水线]
    A --> M[ModifyGraph] --> |ctx,id,modifications| N[原子图修改]
    A --> O[UpdateConfig] --> |ctx,id,newConfig| P[配置比对更新]
    A --> Q[ExportConfig] --> |id| R[导出YAML配置]
    A --> S[Rm] --> |id| T[移除记录]
    end
```

## RuntimeImpl 结构

```mermaid
flowchart TD
    subgraph RuntimeImpl
    A[RuntimeImpl] --> B[pipelines<br/>map string Pipeline]
    A --> C[pipelineIds<br/>map string bool]
    A --> D[pipelineConfigs<br/>map string PipelineConfig]
    A --> E[mu<br/>sync.RWMutex]
    A --> F[ctx<br/>context.Context]
    A --> G[cancel<br/>context.CancelFunc]
    A --> H[doneChan<br/>chan struct]
    A --> I[background<br/>chan struct]
    A --> J[pusher<br/>logger.Pusher]
    A --> K[templateEngine<br/>template.TemplateEngine]
    end
```

## RunAsync 执行流程

```mermaid
flowchart TD
    A[RunAsync] --> B[获取templateEngine]
    B --> C[Lock mu]
    C --> D{ID已存在?}
    D -->|是| E[Unlock<br/>return err]
    D -->|否| F[parseConfig<br/>解析YAML]
    F --> G{parse成功?}
    G -->|否| H[Unlock<br/>return err]
    G -->|是| I[renderConfig<br/>渲染配置]

    I --> J{render成功?}
    J -->|否| K[Unlock<br/>return err]
    J -->|是| L[NewPipeline<br/>创建流水线]

    L --> M[SetTemplateEngine]
    M --> N[SetPusher]
    N --> O[Listening<br/>设置监听器]
    O --> P[buildGraph<br/>构建图结构]
    P --> Q[SetGraph<br/>设置图]

    Q --> R[SetParam<br/>设置渲染后的参数]
    R --> S[SetMaxLoopIterations<br/>设置最大迭代次数]
    S --> T[setupMetadata<br/>设置元数据]

    T --> U[NewProvider<br/>创建执行器提供者]
    U --> V{遍历Executors}
    V --> W[RegisterExecutor<br/>注册每个执行器]
    W --> V
    V -->|完成| X[SetExecutorProvider]

    X --> Y[存储pipeline<br/>标记ID已用<br/>存储config]
    Y --> Z[异步goroutine<br/>执行pipeline.Run]

    Z --> AA[Unlock]
    AA --> AB[return pipeline]
```

## renderConfig 配置渲染

```mermaid
flowchart TD
    A[renderConfig] --> B[转换Param<br/>map string interface<br/>to map string FieldItem]
    B --> C[构建ctx<br/>包含Param值]
    C --> D[ctx["Param"] = ctx<br/>自引用]

    D --> E{param非空?}
    E -->|是| F[renderParam<br/>迭代渲染]
    E -->|否| G[跳过Param渲染]

    F --> H{达到最大迭代<br/>或收敛?}
    H -->|是| I[继续]
    H -->|否| F
    G --> J[更新config.Param<br/>值]

    J --> K{metadata非空?}
    K -->|是| L[转换Metadata.Data<br/>为FieldItem]
    K -->|否| Z[return]
    L --> M[构建ctx<br/>包含Param]
    M --> N[renderMetadata<br/>渲染metadata]
    N --> O[更新config<br/>MetadateData]
    O --> Z
```

## renderParam 迭代渲染

```mermaid
flowchart TD
    A[renderParam] --> B{maxIterations = 10}
    B --> C{changed &&<br/>iteration < max}
    C -->|是| D[遍历result]
    D --> E[构建ctx<br/>包含所有Param值]
    E --> F[renderValue<br/>递归渲染值]

    F --> G{值变化?}
    G -->|是| H[标记changed=true<br/>更新result]
    G -->|否| I[继续下一key]
    H --> I
    I --> D
    D -->|遍历完成| C

    C -->|否| J{未收敛?}
    J -->|是| K[return err<br/>循环引用检测]
    J -->|否| Z[return result]
```

## buildGraph 图构建

```mermaid
flowchart TD
    A[buildGraph] --> B[NewDGAGraph]
    B --> C{遍历config.Nodes}
    C --> D[创建DGANode]
    D --> E[EnsureIds<br/>确保步骤有ID]
    E --> F[AddVertex<br/>添加节点到图]
    F --> C

    C -->|完成| G{config.Graph<br/>非空?}
    G -->|否| Z[return graph]
    G -->|是| H[parseGraphEdges]

    H --> I[NewStateParser]
    I --> J[Parse图字符串]
    J --> K{解析成功?}
    K -->|否| Z
    K -->|是| L[遍历Statements]

    L --> M{是Transition?}
    M -->|是| N{from is "[*]"?}
    N -->|是| O[AddEntryNode]
    N -->|否| P{to is "[*]"?}
    P -->|是| Q[AddExitNode]
    P -->|否| R[提取条件表达式]

    R --> S{有条件?}
    S -->|是| T[NewConditionalEdge]
    S -->|否| U[NewDGAEdge]
    T --> V[AddEdge]
    U --> V
    V --> L
    M -->|否| L
```

## parseGraphEdges 边解析

```mermaid
flowchart TD
    A[parseGraphEdges] --> B[遍历Transition]
    B --> C{from is "[*]"?}
    C -->|是| D[AddEntryNode]
    D --> E[continue]
    C -->|否| F{to is "[*]"?}
    F -->|是| G[AddExitNode]
    G --> E
    F -->|否| H[查找源和目标节点]
    H --> I{节点都存在?}
    I -->|否| B
    I -->|是| J[extractExpression<br/>提取条件]

    J --> K{有条件表达式?}
    K -->|是| L[NewConditionalEdge]
    K -->|否| M[NewDGAEdge]
    L --> N[AddEdge]
    M --> N
    N --> B
```

## ModifyGraph 图修改

```mermaid
flowchart TD
    A[ModifyGraph] --> B[获取pipeline<br/>和config]
    B --> C{可修改?}
    C -->|否| D[return err]
    C -->|是| E[快照当前状态<br/>用于回滚]

    E --> F{删除边}
    F --> G{遍历RemoveEdges}
    G --> H[RemoveEdge]
    H --> I{失败?}
    I -->|是| J[rollback<br/>return err]
    I -->|否| G
    G -->|完成| K{删除节点}
    K --> L{遍历RemoveNodes}
    L --> M[RemoveVertex]
    M --> N{失败?}
    N -->|是| J
    N -->|否| L
    L -->|完成| O{添加节点}
    O --> P[遍历AddNodes]
    P --> Q[创建DGANode]
    Q --> R[AddVertex]
    R --> P
    P -->|完成| S{添加边}
    S --> T[遍历AddEdges]
    T --> U[创建Edge]
    U --> V[AddEdge]
    V --> T
    T -->|完成| W{AddGraph<br/>非空?}
    W -->|是| X[parseGraphEdges]
    W -->|否| Y[校验图结构]
    X --> Y
    Y --> Z{有环?}
    Z -->|无条件环| AA[rollback<br/>return err]
    Z -->|无环或条件环| AB[更新config.Nodes]
    AB --> AC[触发事件<br/>PipelineGraphModified]
    AC --> Z2[return nil]
```

## UpdateConfig 配置更新

```mermaid
flowchart TD
    A[UpdateConfig] --> B[获取pipeline<br/>和oldConfig]
    B --> C{可修改?}
    C -->|否| D[return err]
    C -->|是| E[parse新配置]

    E --> F{parse成功?}
    F -->|否| G[return err]
    F -->|是| H[validateImmutableFields<br/>校验不可变字段]

    H --> I{校验通过?}
    I -->|否| J[return err]
    I -->|是| K[computeNodeModifications<br/>计算节点差异]

    K --> L{Graph定义变化?}
    L -->|是| M[收集所有旧边<br/>到RemoveEdges]
    M --> N[设置新AddGraph]
    L -->|否| O{有节点差异?}
    O -->|是| P[ModifyGraph]
    O -->|否| Q{Graph变化?}
    Q -->|是| P
    Q -->|否| Z[return nil]
    P --> R[更新config.Graph]
    R --> Z
```

## Pause/Resume 暂停恢复

```mermaid
flowchart TD
    subgraph Pause
    A[Pause] --> B[Lock pauseMu]
    B --> C[获取status]
    C --> D{status is<br/>RUNNING?}
    D -->|否| E[Unlock<br/>return err]
    D -->|是| F[close pauseChan<br/>发送暂停信号]
    F --> G[Unlock<br/>return nil]
    end

    subgraph Resume
    H[Resume] --> I[Lock pauseMu]
    I --> J[获取status]
    J --> K{status is<br/>PAUSED?}
    K -->|否| L[Unlock<br/>return err]
    K -->|是| M[close resumeChan<br/>发送恢复信号]
    M --> N[Unlock<br/>return nil]
    end
```

## 后台清理机制

```mermaid
flowchart TD
    A[StartBackground] --> B[goroutine]
    B --> C[ticker = 30秒]
    C --> D{select}
    D --> E{ctx.Done?}
    E -->|是| Z[return]
    E -->|否| F{定时器触发}
    F --> G[cleanup<br/>CompletedPipelines]

    G --> H[Lock mu]
    H --> I{遍历pipelines}
    I --> J{pipeline.Done?}
    J -->|是| K[delete<br/>清理记录]
    J -->|否| L[跳过]
    K --> I
    I -->|完成| M[Unlock]
    M --> C
```

## 配置导出

```mermaid
flowchart TD
    A[ExportConfig] --> B[Lock RLock]
    B --> C[获取pipeline<br/>和config]
    C --> D{都存在?}
    D -->|否| E[Unlock<br/>return err]
    D -->|是| F[NewPipeline<br/>Snapshotter]

    F --> G[TakeSnapshot<br/>生成带状态配置]
    G --> H{成功?}
    H -->|否| I[Unlock<br/>return err]
    H -->|是| J[ToYAML<br/>转换为YAML]

    J --> K[Unlock<br/>return yamlStr]
```