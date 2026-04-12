# 执行器系统

本文档介绍 Pipelinex 的执行器架构：Executor 接口、Provider 工厂模式，以及 Local、Docker、Kubernetes 三种执行后端。

## 核心接口

### Executor 接口

```go
type Executor interface {
    Prepare(ctx context.Context) error              // 准备执行环境
    Destruction(ctx context.Context) error           // 销毁执行环境
    Transfer(ctx, resultChan, commandChan, inputChan) // 命令循环
}
```

**生命周期：**

```
Prepare → Transfer（可多次执行命令）→ Destruction
```

### Adapter 接口

```go
type Adapter interface {
    Config(ctx context.Context, config map[string]any) error  // 应用配置
}
```

### Bridge 接口

```go
type Bridge interface {
    Conn(ctx context.Context, adapter Adapter) (Executor, error)  // 创建执行器
}
```

### ExecutorInfoProvider 接口

```go
type ExecutorInfoProvider interface {
    GetRuntimeInfo() map[string]any   // 运行时信息
    GetInstanceId() string             // 实例 ID
    GetType() string                   // 类型标识
}
```

### 通道类型

```go
// 命令通道：发送要执行的命令
type CommandWrapper struct {
    StepName string
    Command  string
}

// 结果通道：接收执行结果
type StepResult struct {
    StepName   string
    Command    string
    Output     []byte
    Error      error
    StartTime  time.Time
    FinishTime time.Time
}
```

## Provider（执行器工厂）

Provider 使用注册的执行器配置，通过 Adapter/Bridge 模式创建执行器实例。

```go
provider := provider.NewProvider()

// 注册执行器
provider.RegisterExecutor("local", executor.ExecutorConfig{
    Type:   "local",
    Config: map[string]any{"shell": "bash"},
})
provider.RegisterExecutor("docker", executor.ExecutorConfig{
    Type:   "docker",
    Config: map[string]any{"registry": "myregistry.com"},
})

// 创建并准备执行器
exec, err := provider.GetExecutor(ctx, "docker")
```

**支持的类型字符串：**

| 类型 | 说明 |
|------|------|
| `local` | 本地 Shell 执行 |
| `docker` | Docker 容器执行 |
| `kubernetes` / `k8s` | Kubernetes Pod 执行 |

**创建流程：**

```
RegisterExecutor(name, config)
    ↓
GetExecutor(ctx, name)
    ↓
1. 根据 type 选择 Adapter 和 Bridge
2. Adapter.Config() 应用配置
3. Bridge.Conn() 创建 Executor
4. Executor.Prepare() 初始化环境
    ↓
返回准备好的 Executor
```

---

## Local 执行器

在本地 Shell 环境中执行命令。

### 创建

```go
exec := local.NewLocalExecutor()
```

### 配置项

| 配置键 | 类型 | 说明 |
|--------|------|------|
| `workdir` | string | 工作目录，必须存在 |
| `env` | map[string]string | 环境变量 |
| `shell` | string | Shell 类型（bash/sh/zsh） |
| `timeout` | string/int | 单命令超时（如 `"60"` 或 `"2m"`） |
| `pty` | bool | 是否使用 PTY |
| `ptyWidth` | int | PTY 宽度 |
| `ptyHeight` | int | PTY 高度 |

### Shell 自动检测

- Unix：优先 `bash`，回退 `sh`
- Windows：优先 `pwsh`/`powershell`，回退 `cmd`

### 行为特点

- `Prepare`：验证工作目录存在、配置的 Shell 可用
- `Transfer`：通过 `os/exec` 执行命令，实时流式输出 stdout 和 stderr
- **交互式输入**：支持通过 `inputChan` 向进程 stdin 发送数据，可识别输出中的 `flowx-input` 代码块
- **取消**：先发送 `os.Interrupt`，不响应则 `Process.Kill()`
- **超时**：支持单命令级别超时

---

## Docker 执行器

在 Docker 容器中执行命令。

### 创建

```go
// 使用环境 Docker 客户端
exec, err := docker.NewDockerExecutor()

// 注入自定义客户端
exec := docker.NewDockerExecutorWithClient(cli)
```

### 配置项

| 配置键 | 类型 | 说明 |
|--------|------|------|
| `registry` | string | 默认镜像仓库前缀 |
| `network` | string | 容器网络模式 |
| `workdir` | string | 容器内工作目录 |
| `volumes` | []string | 挂载卷列表，格式 `"host:container"` |
| `env` | map[string]string | 环境变量 |
| `tty` | bool | 启用 TTY |
| `ttyWidth` | int | TTY 宽度 |
| `ttyHeight` | int | TTY 高度 |

### 行为特点

- `Prepare`：
  - 创建容器（初始命令 `sleep 3600`）
  - 如果镜像不存在本地，自动拉取
  - 启动容器，等待最多 3 秒确认 running 状态
- `Transfer`：
  - 通过 Docker `exec` API 在容器内执行命令
  - 实时流式输出
  - 监听 context 取消，终止当前 exec
- **Shell 检测**：Alpine/Busybox 镜像使用 `/bin/sh`，其他使用 `/bin/bash`
- **隔离性**：每个执行器对应一个独立的容器实例
- **容器复用**：同一个执行器可执行多个命令（容器持续运行）

### 示例配置

```yaml
Executors:
  docker:
    type: docker
    config:
      registry: myregistry.com
      network: host
      workdir: /app
      tty: true
      volumes:
        - /var/run/docker.sock:/var/run/docker.sock
        - ${PWD}:/workspace
      env:
        GO_VERSION: "1.21"
```

---

## Kubernetes 执行器

在 Kubernetes Pod 中执行命令。

### 创建

```go
// 使用环境 kubeconfig
exec, err := kubernetes.NewKubernetesExecutor()

// 注入 REST 配置
exec, err := kubernetes.NewKubernetesExecutorWithConfig(restConfig)

// 注入所有依赖
exec := kubernetes.NewKubernetesExecutorWithClient(client, restConfig, "default")
```

### 配置项

| 配置键 | 类型 | 说明 |
|--------|------|------|
| `namespace` | string | K8s 命名空间 |
| `image` | string | 容器镜像 |
| `workdir` | string | 工作目录 |
| `env` | map[string]string | 环境变量 |
| `serviceAccount` | string | ServiceAccount 名称 |
| `configMaps` | []map | ConfigMap 挂载 |
| `secrets` | []map | Secret 挂载 |
| `volumes` | []map | PVC 挂载 |
| `emptyDirs` | []map | EmptyDir 挂载 |
| `tty` | bool | 启用 TTY |
| `ttyWidth` | int | TTY 宽度 |
| `ttyHeight` | int | TTY 高度 |
| `podReadyTimeout` | int | Pod 就绪超时秒数（默认 60） |
| `resources` | map | 资源限制（cpu/memory） |

### 资源配置

```yaml
config:
  resources:
    cpu: "1000m"        # CPU 限制
    memory: "2Gi"       # 内存限制
```

### 卷挂载

```yaml
config:
  # ConfigMap
  configMaps:
    - name: my-config
      mountPath: /etc/config
  # Secret
  secrets:
    - name: my-secret
      mountPath: /etc/secrets
  # PVC
  volumes:
    - name: data-volume
      mountPath: /data
  # EmptyDir
  emptyDirs:
    - name: temp-data
      mountPath: /tmp/data
```

### 行为特点

- `Prepare`：
  - 创建 Pod（初始命令 `sleep 3600`）
  - 等待 Pod 进入 Running 状态（默认超时 60 秒）
- `Transfer`：
  - 通过 Kubernetes `exec` API（SPDY 协议）在 Pod 内执行命令
  - 实时流式输出
  - 取消时通过 stdin pipe 发送 `Ctrl+C`（`\x03`）终止远程进程
- **Destruction**：
  - 使用独立的 background context（30 秒超时）确保清理
  - 优雅终止期为 10 秒
- **状态**：Pod 在执行期间保持运行，销毁时被删除

### 运行时信息

```go
info := exec.GetRuntimeInfo()
// map[string]any{
//     "podName":   "pipeline-xxx",
//     "namespace": "default",
//     "image":     "golang:1.21",
// }
```
