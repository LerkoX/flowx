# Executor 模块流程图

## 接口层次

```mermaid
flowchart TD
    subgraph ExecutorProvider
    A[ExecutorProvider] --> B[GetExecutor<br/>name → Executor]
    end

    subgraph Executor
    C[Executor] --> D[Prepare<br/>准备环境]
    C --> E[Transfer<br/>执行命令]
    C --> F[Destruction<br/>销毁环境]
    end

    subgraph ExecutorInfoProvider
    G[ExecutorInfoProvider] --> H[GetRuntimeInfo<br/>运行时信息]
    G --> I[GetInstanceId<br/>实例ID]
    G --> J[GetType<br/>执行器类型]
    end

    subgraph Bridge & Adapter
    K[Bridge] --> L[Conn<br/>连接环境]
    L --> M[Adapter]
    M --> N[Config<br/>配置]
    end
```

## ExecutorProvider 工厂

```mermaid
flowchart TD
    A[Provider] --> B[executors<br/>map string ExecutorConfig]

    A --> C[RegisterExecutor]
    C --> D[保存配置到map]

    A --> D2[GetExecutor]
    D2 --> E{查找配置}
    E -->|不存在| F[return nil<br/>err]
    E -->|存在| G{创建执行器}

    G --> H{Type is "local"}

    G --> I{Type is "docker"}

    G --> J{Type is "ssh"}

    H --> K[NewLocalExecutor]
    I --> L[NewDockerExecutor]
    J --> M[NewSSHExecutor]

    K --> N[配置执行器]
    L --> N
    M --> N
    N --> O[return executor]
```

## LocalExecutor 本地执行器

```mermaid
flowchart TD
    subgraph Prepare
    A[Prepare] --> B{workdir存在?}
    B -->|是| C[验证是目录]
    C -->|无效| D[return err]
    C -->|有效| E[验证shell可用]
    E --> F[return nil]
    B -->|否| F
    end

    subgraph Destruction
    G[Destruction] --> H[killCurrentProcess<br/>终止当前进程]
    H --> I[return nil]
    end

    subgraph Transfer
    J[Transfer] --> K[创建可取消context]
    K --> L[启动goroutine<br/>监听外部取消]
    L --> M{遍历commandChan}

    M --> N[获取CommandWrapper]
    N --> O[executeCommand<br/>Streaming]
    O --> P[发送实时输出<br/>resultChan <- data]

    P --> Q[发送StepResult<br/>resultChan <- result]
    Q --> M

    M -->|channel关闭| R[return]
    end
```

## LocalExecutor 命令执行

```mermaid
flowchart TD
    A[executeCommandStreaming] --> B[记录startTime]
    B --> C{有timeout?}
    C -->|是| D[创建TimeoutContext]
    C -->|否| E[使用原context]

    D --> E
    E --> F[创建inputRequestChan]
    F --> G[启动goroutine<br/>处理输入请求]

    G --> H[executeCommand<br/>WithStreaming]

    H --> I[读取stdout/stderr]
    I --> J{检测到输入请求?}
    J -->|是| K[发送InputRequestEvent]
    J -->|否| L[回调输出]
    K --> M[等待inputChan]
    M --> N[写入stdin]
    L --> I

    H --> O[命令结束]
    O --> P[发送StepResult]
    P --> Q[包含StartTime<br/>FinishTime<br/>Error]
```

## LocalExecutor PTY模式

```mermaid
flowchart TD
    A[PTY模式] --> B[创建PTY]
    B --> C[exec.Command<br/>Start]
    C --> D[设置stdin<br/>stdout<br/>stderr]
    D --> E[循环读取输出]
    E --> F[回调输出]
    F --> G{收到输入请求?}
    G -->|是| H[InputRequestEvent]
    H --> I[从inputChan读取]
    I --> J[写入PTY]
    J --> E
    G -->|否| E
    E -->|结束| K[return]
```

## DockerExecutor

```mermaid
flowchart TD
    subgraph Prepare
    A[Prepare] --> B[连接Docker Client]
    B --> C{连接成功?}
    C -->|否| D[return err]
    C -->|是| E[拉取镜像]
    E --> F{镜像存在?}
    F -->|否| G[PullImage]
    F -->|是| H[创建容器]
    G --> H
    H --> I[StartContainer]
    I --> J[等待容器就绪]
    J --> K[return nil]
    end

    subgraph Transfer
    L[Transfer] --> M[创建Exec]
    M --> N[StartExecStream<br/>流式执行]
    N --> O[实时输出回调]
    O --> P[处理输入请求]
    P --> Q[发送StepResult]
    end

    subgraph Destruction
    R[Destruction] --> S[StopContainer]
    S --> T[RemoveContainer]
    T --> U[Close Client]
    end
```

## DockerExecutor 交互式执行

```mermaid
flowchart TD
    A[交互式执行] --> B[创建stdin连接]
    B --> C[创建callbackWriter]
    C --> D[StartExec<br/>with TTY]

    D --> E[goroutine:<br/>读取容器输出]
    E --> F[检测输入请求]
    F --> G{有输入请求?}
    G -->|是| H[发送<br/>InputRequestEvent]
    H --> I[等待inputChan]
    I --> J[写入stdin]
    J --> E
    G -->|否| K[回调输出]
    K --> E

    E -->|输出结束| L[发送StepResult]
```

## KubernetesExecutor

```mermaid
flowchart TD
    subgraph Prepare
    A[Prepare] --> B[创建K8s Client]
    B --> C[创建Pod Spec]
    C --> D[CreatePod]
    D --> E[等待Pod就绪]
    E --> F{超时?}
    F -->|是| G[DeletePod<br/>return err]
    F -->|否| H{Pod就绪?}
    H -->|否| E
    H -->|是| I[获取Pod信息]
    I --> J[return nil]
    end

    subgraph Transfer
    K[Transfer] --> L[创建Exec请求]
    L --> M[ConnectExec<br/>流式执行]
    M --> N[实时输出]
    N --> O[处理输入]
    O --> P[发送StepResult]
    end

    subgraph Destruction
    Q[Destruction] --> R[DeletePod]
    R --> S[return nil]
    end
```

## 执行器状态

```mermaid
stateDiagram-v2
    [*] --> PREPARED: Prepare
    PREPARED --> RUNNING: Transfer开始
    RUNNING --> DESTROYED: Destruction
    DESTROYED --> [*]

    note right of PREPARED: 环境准备就绪
    note right of RUNNING: 执行命令中
    note right of DESTROYED: 环境已销毁
```

## 命令执行结果

```mermaid
flowchart TD
    A[StepResult] --> B[StepName<br/>步骤名称]
    A --> C[Command<br/>执行的命令]
    A --> D[Output<br/>标准输出]
    A --> E[Error<br/>错误信息]
    A --> F[StartTime<br/>开始时间]
    A --> G[FinishTime<br/>结束时间]

    subgraph 状态判断
    H{Error != nil?}
    H -->|是| I[Status = FAILED]
    H -->|否| J[Status = SUCCESS]
    end
```

## 输入请求流程

```mermaid
flowchart TD
    A[检测到输入请求] --> B[程序输出<br/>{"flowx":"wait-input"}]
    B --> C[解析请求信息]
    C --> D[Prompt<br/>提示信息]
    C --> E[Type<br/>text/password/confirm]
    C --> F[Timeout<br/>超时时间]

    D --> G[创建<br/>InputRequestEvent]
    G --> H[发送到resultChan]
    H --> I[外部处理<br/>等待用户输入]

    I --> J[InputReadyEvent]
    J --> K[发送到inputChan]
    K --> L[继续执行]
```

## CommandWrapper 命令包装

```mermaid
flowchart TD
    A[CommandWrapper] --> B[StepName<br/>步骤名称<br/>用于映射结果]
    A --> C[Command<br/>待执行命令<br/>可能是模板]

    subgraph 命令发送
    D[Pipeline] --> E[renderString<br/>渲染命令模板]
    E --> F[CommandWrapper<br/>stepName + command]
    F --> G[commandChan <- wrapper]
    G --> H[Executor接收]
    end

    subgraph 结果映射
    I[Executor] --> J[StepResult<br/>包含StepName]
    J --> K[Pipeline根据StepName<br/>匹配到对应步骤]
    K --> L[更新步骤状态]
    end
```

## Provider 注册机制

```mermaid
flowchart TD
    A[RegisterExecutor] --> B[name]
    A --> C[ExecutorConfig]
    C --> D[Type: 执行器类型]
    C --> E[Config: 配置映射]
    C --> F[Description: 描述]

    B --> G[executors[name] = config]

    A2[GetExecutor] --> H{有缓存?}
    H -->|有| I[返回缓存实例]
    H -->|无| J{有配置?}
    J -->|无| K[return err]
    J -->|有| L{Type?}
    L -->|local| M[NewLocalExecutor]
    L -->|docker| N[NewDockerExecutor]
    L -->|k8s| O[NewK8sExecutor]
    L -->|ssh| P[NewSSHExecutor]
    M --> Q[Config配置]
    N --> Q
    O --> Q
    P --> Q
    Q --> R[缓存并返回]
```