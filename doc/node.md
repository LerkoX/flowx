# 节点与步骤

本文档介绍节点（Node）和步骤（Step）的概念、配置、状态管理以及输出提取功能。

## Node 接口

```go
type Node interface {
    Id() string                                        // 节点唯一标识
    PipelineId() string                                // 所属 Pipeline ID
    Status() string                                    // 节点状态
    Get(key string) string                             // 读取属性
    Set(key string, value any)                         // 写入属性
    GetExecutor() string                               // 使用的执行器名称
    GetSteps() []Step                                  // 获取步骤列表
    GetConfig() map[string]any                         // 节点配置
    GetRuntimeStatus() *NodeRuntimeStatus              // 运行时状态
    SetRuntimeStatus(*NodeRuntimeStatus)               // 更新运行时状态
    GetStepRuntimeStatus(stepName string) *StepRuntimeStatus  // 步骤运行时状态
    SetStepRuntimeStatus(*StepRuntimeStatus)           // 更新步骤运行时状态
    EnsureIds()                                        // 确保 ID 已分配
}
```

## DGANode 实现

### 构造函数

```go
// 最小构造
node := NewDGANode("Build", "")

// 完整构造
node := NewDGANodeWithConfig(
    "Build",           // ID
    "",                // 状态
    "docker",          // 执行器名称
    "golang:1.21",     // 镜像
    steps,             // 步骤列表
    config,            // 配置
)
```

## 节点配置

```yaml
Nodes:
  Build:
    name: "构建应用"                    # 显示名称
    description: "编译并构建镜像"        # 功能描述
    executor: docker                   # 引用的执行器名称
    image: golang:1.21-alpine          # 容器镜像（Docker/K8s）
    steps:                             # 步骤列表
      - name: compile
        run: go build -o app .
      - name: image
        run: docker build -t app:latest .
    extract:                           # 输出提取配置（可选）
      type: codec-block
```

## 步骤（Step）

### Step 结构体

```go
type Step struct {
    Id          string  // 步骤唯一标识（由 EnsureIds() 自动生成，无需手动设置）
    Name        string  // 步骤名称
    Description string  // 步骤描述
    Run         string  // 执行的 shell 命令（支持模板渲染）
}
```

### 多步骤执行

节点支持多个步骤按顺序执行。每个步骤的 `run` 命令在执行时会通过模板引擎渲染，可以引用 `Param` 和 `Metadata` 中的值。

```yaml
Nodes:
  Deploy:
    executor: k8s
    image: bitnami/kubectl:latest
    steps:
      - name: apply
        description: "应用 K8s 资源配置"
        run: kubectl apply -f ./k8s/
      - name: verify
        description: "等待滚动更新完成"
        run: kubectl rollout status deployment/app -n {{ Param.namespace }}
      - name: report
        description: "输出部署信息"
        run: echo "Deployed {{ Param.fullImage }} to {{ Param.namespace }}"
```

### 步骤跳过

在恢复执行时，已完成的步骤会被跳过。引擎通过 `StepRuntimeStatus` 判断步骤状态：

- 状态为 `SUCCESS`、`FAILED`、`CANCELLED` 的步骤会被跳过
- 输出日志中会显示 "Skipping step xxx (status: SUCCESS)"

## 状态管理

### NodeRuntimeStatus

```go
type NodeRuntimeStatus struct {
    Id         string                  // 运行时 ID
    Status     string                  // 节点状态
    StartTime  time.Time               // 开始时间
    EndTime    time.Time               // 结束时间
    Steps      []StepRuntimeStatus     // 步骤状态列表
    Executor   *ExecutorRuntimeInfo    // 执行器信息
    Custom     map[string]interface{}  // 自定义数据
}
```

### StepRuntimeStatus

```go
type StepRuntimeStatus struct {
    Id        string        // 步骤 ID
    Name      string        // 步骤名称
    Status    string        // 步骤状态
    StartTime time.Time     // 开始时间
    EndTime   time.Time     // 结束时间
    Error     string        // 错误信息
    Output    string        // 输出内容
}
```

### ExecutorRuntimeInfo

```go
type ExecutorRuntimeInfo struct {
    Type       string                // 执行器类型
    InstanceId string                // 实例 ID（容器ID/Pod名）
    Status     string                // 执行器状态
    Info       map[string]interface{} // 详细信息
}
```

### 状态枚举

| 状态 | 含义 |
|------|------|
| `Pending` | 等待执行 |
| `Running` | 执行中 |
| `SUCCESS` | 执行成功 |
| `FAILED` | 执行失败 |
| `CANCELLED` | 已取消 |

## 输出提取

输出提取功能可以从命令输出中提取结构化数据，保存到 metadata 中供后续节点使用。提取的数据会自动填充 `srcNode` 字段。

### 提取器内部实现

输出提取由 Pipeline 内部方法 `extractOutput()` 处理，通过 `createExtractor()` 根据配置创建对应的提取器：

```go
// PipelineImpl 内部方法（非公开 API）
func (p *PipelineImpl) extractOutput(ctx context.Context, node Node, stepResult *StepResult, fullOutput string) error
func (p *PipelineImpl) createExtractor(extractConfig interface{}) (OutputExtractor, error)
```

提取器接口（内部使用）：

```go
type OutputExtractor interface {
    Extract(output string) (map[string]core.FieldItem, error)
}
```

### codec-block 模式

识别输出中的 `flowx-yaml` 代码块并解析，支持行尾注释提取 description。

**配置：**

```yaml
extract:
  type: codec-block
  maxOutputSize: 1048576  # 1MB，可选
```

**在命令中嵌入（flowx-yaml）：**

```bash
echo '```flowx-yaml'
echo 'version: "1.0.0"  # 版本号'
echo 'buildStatus: "success"  # 构建状态'
echo 'imageTag: "myapp:v1.0.0"  # 镜像标签'
echo '```'
```

提取后的数据会自动设置 `srcNode` 为当前节点 ID，并从注释中提取 `description`：

```go
// 提取结果示例
map[string]core.FieldItem{
    "version": {Value: "1.0.0", Description: "版本号", SrcNode: "Build"},
    "buildStatus": {Value: "success", Description: "构建状态", SrcNode: "Build"},
    "imageTag": {Value: "myapp:v1.0.0", Description: "镜像标签", SrcNode: "Build"},
}
```

存储到 Metadata 时会添加节点前缀：`Metadata["Build.version"]`

### regex 模式

使用正则表达式提取内容，每个正则对应一个 key。

**配置：**

```yaml
extract:
  type: regex
  patterns:
    coverage: "coverage: (\\d+\\.\\d+)%"     # 提取测试覆盖率
    testsPassed: "(\\d+) tests passed"        # 提取通过的测试数
    buildStatus: "Build (\\w+)"               # 提取构建状态
  maxOutputSize: 524288                       # 512KB
```

**规则：**

- 正则表达式可以包含捕获组，第一个捕获组的内容作为值
- 如果没有捕获组，使用完整匹配作为值
- 如果正则无效，创建时返回错误

### 提取的数据存储

提取的数据会经过 `convertToString()` 转换后存储：
- **字符串**：原样保存
- **slice/map**：序列化为 JSON 字符串保存
- **其他类型**：使用 `fmt.Sprintf("%v", val)` 转换

这使得复杂类型（如数组）可以在后续节点中通过 `tryParseJSON()` 自动解析回对象。

## 交互式输入

Local 执行器支持程序主动请求输入。当输出中包含 `flowx-input` 代码块时，执行器会发出输入请求事件。

**输出格式：**

```
```flowx-input
prompt: "请输入部署目标"
type: text
timeout: 30
```
```

**输入类型：**

| type | 说明 |
|------|------|
| `text` | 普通文本输入 |
| `password` | 密码输入 |
| `confirm` | 确认输入（y/n） |

---

## flowx.json 节点注册（Studio 扩展）

FlowX Studio 支持通过 `flowx.json` 文件注册节点到节点注册中心。每个 `flowx.json` 可以声明一个或多个节点，包含完整的执行元数据。

### flowx.json 结构

```json
{
  "name": "image-resizer",
  "displayName": "图片缩放器",
  "description": "将图片缩放到指定尺寸",
  "version": "1.0.0",
  "author": "flowx-team",
  "tags": ["image", "resize"],
  "icon": "🖼️",
  "executor": {
    "type": "local",
    "workdir": "./nodes/image-resizer",
    "entry": "main.py",
    "language": "python"
  },
  "parameters": [
    {
      "name": "input_path",
      "type": "string",
      "description": "输入图片路径",
      "required": true
    }
  ],
  "outputs": [
    {
      "name": "output_path",
      "type": "string",
      "description": "输出图片路径"
    }
  ],
  "paramDelivery": "env"
}
```

### 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 节点唯一标识（英文，用于引用） |
| `displayName` | string | 否 | 节点显示名称 |
| `description` | string | 否 | 节点功能描述 |
| `version` | string | 否 | 版本号 |
| `author` | string | 否 | 作者 |
| `tags` | []string | 否 | 标签列表 |
| `icon` | string | 否 | 图标（emoji 或字符） |
| `executor` | object | 是 | 执行器配置 |
| `executor.type` | string | 是 | 执行器类型：`local`、`docker`、`kubernetes` |
| `executor.workdir` | string | 否 | 工作目录（仅 `local`） |
| `executor.entry` | string | 是 | 执行入口文件或命令 |
| `executor.language` | string | 条件 | 执行语言（仅 `local` 必填） |
| `executor.image` | string | 条件 | 容器镜像（`docker`/`kubernetes` 必填） |
| `parameters` | []object | 否 | 输入参数定义 |
| `outputs` | []object | 否 | 输出字段定义 |
| `paramDelivery` | string | 否 | 参数传递方式：`env`（默认）、`args`、`stdin` |

### 多节点声明

一个 `flowx.json` 可以声明多个节点（数组形式）：

```json
[
  {
    "name": "image-resizer",
    "displayName": "图片缩放器",
    "executor": { "type": "local", "entry": "main.py", "language": "python" },
    "parameters": [...],
    "outputs": [...]
  },
  {
    "name": "data-processor",
    "displayName": "数据处理器",
    "executor": { "type": "docker", "image": "myregistry/data-processor:v1", "entry": "python /app/main.py" },
    "parameters": [...],
    "outputs": [...]
  }
]
```

### 参数传递方式

`paramDelivery` 决定节点执行时如何接收参数：

| 方式 | 说明 |
|------|------|
| `env` | 参数作为环境变量注入（默认） |
| `args` | 参数作为命令行参数 `--key=value` 传递 |
| `stdin` | 参数序列化为 JSON 写入标准输入 |

### 与 YAML 配置的关系

`flowx.json` 是节点注册中心的配置格式，用于在 Studio 中注册可复用的节点。注册后的节点可以在 YAML 流水线配置中通过 `executor` 字段引用：

```yaml
Executors:
  image-resizer:
    type: local
    config:
      workdir: ./nodes/image-resizer
      shell: bash

Nodes:
  Resize:
    executor: image-resizer
    steps:
      - name: resize
        run: python main.py
```

> 注：`flowx.json` 中的 `executor` 配置与 YAML 中的 `Executors` 配置是互补的。`flowx.json` 描述节点本身的执行元数据，YAML 中的 `Executors` 描述运行时的执行环境配置。