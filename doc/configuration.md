# 配置参考

Workflowx 使用 YAML 格式定义流水线配置。本文档详细说明所有配置字段。

## 完整结构

```yaml
Version: "1.0"
Name: my-workflow

Metadate:
  type: in-config
  description: "元数据用途说明"
  data:
    key1: value1

AI:
  intent: "流水线业务意图描述"
  constraints:
    - "约束条件"
  template: "template-id"

Param:
  key: value

Executors:
  local:
    type: local
    config: {}

Logging:
  endpoint: http://log-center/api/v1/logs
  headers: {}
  timeout: 5s

MaxLoopIterations: 100

Graph: |
  stateDiagram-v2
    [*] --> Node1
    Node1 --> Node2

Status:
  Node1: Finished

Nodes:
  NodeName:
    executor: local
    image: optional-image
    steps: []
```

---

## 1. Version

| 字段 | 类型 | 说明 |
|------|------|------|
| `Version` | string | 配置文件格式版本，用于引擎兼容性判断 |

---

## 2. Name

| 字段 | 类型 | 说明 |
|------|------|------|
| `Name` | string | 流水线名称，用于标识、日志和监控 |

---

## 3. Metadate（元数据配置）

| 字段 | 类型 | 说明 |
|------|------|------|
| `Metadate.type` | string | 存储类型：`in-config`、`redis`、`http` |
| `Metadate.description` | string | 可选，描述元数据用途 |
| `Metadate.data` | map | 初始元数据键值对，支持引用 `Param` 的模板渲染 |

### FieldItem 结构

Metadate.data 中的每个字段支持两种格式：

1. **简单格式**（推荐用于 AI 生成配置）：
```yaml
Metadate:
  type: in-config
  data:
    # 字段: 值  # 描述
    K8sNamespace: "{{ Param.namespace }}"  # Kubernetes 命名空间
    FullImage: "{{ Param.registry }}/myapp:{{ Param.env }}"  # 完整镜像地址
```

2. **完整格式**（显式指定所有字段）：
```yaml
Metadate:
  type: in-config
  data:
    K8sNamespace:
      value: "{{ Param.namespace }}"
      description: "Kubernetes 命名空间"
      srcNode: ""  # 空表示来自配置，节点产生时会填充节点 ID
```

### 说明

- **value**: 字段的实际值，支持模板渲染
- **description**: 字段描述，用于 AI 生成配置时的参考
- **srcNode**: 来源节点 ID，初始化时为空，节点通过 extract 产生的数据会自动填充

> 更多详情参见 [元数据存储](metadata.md)

---

## 4. AI（AI 智能字段）

| 字段 | 类型 | 说明 |
|------|------|------|
| `AI.intent` | string | 一句话描述流水线业务意图 |
| `AI.constraints` | []string | 关键约束条件列表 |
| `AI.template` | string | 模板标识符，用于复用 |
| `AI.generatedAt` | string | 生成时间戳 |
| `AI.version` | int | 意图版本号 |

### 示例

```yaml
AI:
  intent: "Go 微服务构建并部署到 K8s"
  constraints:
    - "多阶段构建"
    - "非 root 用户运行"
  template: "go-microservice"
  generatedAt: "2024-01-15T10:30:00Z"
  version: 1
```

---

## 5. Param（参数定义）

| 字段 | 类型 | 说明 |
|------|------|------|
| `Param` | map | 全局变量池，支持模板渲染和自引用 |

### FieldItem 结构

Param 中的每个字段支持两种格式：

1. **简单格式**（推荐用于 AI 生成配置）：
```yaml
Param:
  env: "production"  # 部署环境
  appName: "myapp"   # 应用名称
  # 自引用
  namespace: "{{ Param.appName }}-{{ Param.env }}"         # 渲染为: myapp-production
```

2. **完整格式**（显式指定所有字段）：
```yaml
Param:
  env:
    value: "production"
    description: "部署环境"
    srcNode: ""
```

### 特性

- **基本引用**：通过 `{{ Param.xxx }}` 语法引用其他参数
- **自引用**：一个 Param 可以引用另一个 Param 的值
- **嵌套结构**：支持 map 和 list 嵌套
- **未定义变量**：引用未定义变量时，模板表达式保持原样不变
- **Description**：字段后的注释会被提取为 description，用于 AI 生成配置

### 示例

```yaml
Param:
  env: "production"  # 部署环境: dev, staging, 或 production
  appName: "myapp"   # 应用名称
  # 自引用
  namespace: "{{ Param.appName }}-{{ Param.env }}"         # 渲染为: myapp-production
  registry: "myregistry.com"
  # 多层引用
  imageName: "{{ Param.registry }}/{{ Param.appName }}"    # 渲染为: myregistry.com/myapp
  fullImage: "{{ Param.imageName }}:latest"                # 渲染为: myregistry.com/myapp:latest
  # 嵌套结构
  config:
    replicas: 3
    envVars:
      - name: "ENVIRONMENT"
        value: "{{ Param.env }}"
```

> 更多详情参见 [模板引擎](template.md)

---

## 6. Executors（执行器定义）

| 字段 | 类型 | 说明 |
|------|------|------|
| `Executors` | map | 全局执行器注册表，供 Nodes 引用 |
| `Executors.{name}.type` | string | 类型：`local`、`docker`、`kubernetes`（或 `k8s`） |
| `Executors.{name}.description` | string | 可选，执行器用途说明 |
| `Executors.{name}.config` | object | 执行器配置，被 Nodes 继承 |

### 6.1 local 执行器

```yaml
Executors:
  local:
    type: local
    config:
      shell: bash          # 指定 shell（bash/sh/zsh）
      workdir: /tmp/build  # 工作目录
      env:                 # 环境变量
        KEY: value
      timeout: 60          # 超时秒数
      pty: false           # 是否使用 PTY
```

### 6.2 docker 执行器

```yaml
Executors:
  docker:
    type: docker
    config:
      registry: myregistry.com     # 默认镜像仓库
      network: host                # 容器网络模式
      workdir: /app                # 容器工作目录
      volumes:                     # 挂载卷
        - /var/run/docker.sock:/var/run/docker.sock
        - ${PWD}:/workspace
      env:                         # 环境变量
        GO_VERSION: "1.21"
      tty: true                    # 启用 TTY
      ttyWidth: 80                 # TTY 宽度
      ttyHeight: 24                # TTY 高度
```

### 6.3 kubernetes 执行器

```yaml
Executors:
  k8s:
    type: kubernetes
    description: "K8s 执行器"      # 执行器用途说明
    config:
      namespace: default           # K8s 命名空间
      serviceAccount: default      # ServiceAccount
      resources:                   # 资源限制
        cpu: "1000m"
        memory: "2Gi"
      podReadyTimeout: 60          # Pod 就绪超时秒数
      configMaps:                  # ConfigMap 挂载
        - name: my-config
          mountPath: /etc/config
      secrets:                     # Secret 挂载
        - name: my-secret
          mountPath: /etc/secrets
      env:
        KEY: value
```

> 更多详情参见 [执行器系统](executor.md)

---

## 7. Logging（日志配置）

| 字段 | 类型 | 说明 |
|------|------|------|
| `Logging.description` | string | 日志配置描述 |
| `Logging.endpoint` | string | 日志接收服务 HTTP 接口地址 |
| `Logging.headers` | map | 请求头（认证、租户标识等） |
| `Logging.timeout` | duration | 单次推送超时时间 |
| `Logging.retry` | int | 推送失败重试次数 |

### 示例

```yaml
Logging:
  endpoint: http://log-center/api/v1/logs
  headers:
    Authorization: Bearer xxxxxxxxx
  timeout: 5s
  retry: 3
```

---

## 8. Graph（流程定义）

| 字段 | 类型 | 说明 |
|------|------|------|
| `Graph` | string | Mermaid `stateDiagram-v2` 语法，定义节点执行顺序和依赖关系 |

### 基本语法

```yaml
Graph: |
  stateDiagram-v2
    [*] --> Build
    Build --> Test
    Test --> Deploy
    Deploy --> [*]
```

### 条件边

边的标签中可以嵌入 pongo2 表达式作为条件：

```yaml
Graph: |
  stateDiagram-v2
    [*] --> Check
    Check --> Deploy : {{ Param.env == "production" }}
    Check --> Skip   : {{ Param.env != "production" }}
```

> 更多详情参见 [条件边](edge.md)

---

## 8.5 MaxLoopIterations（循环迭代上限）

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `MaxLoopIterations` | int | 100 | 循环图最大迭代次数，超过则返回错误 |

### 示例

```yaml
MaxLoopIterations: 5

Graph: |
  stateDiagram-v2
    [*] --> A
    A --> B
    B --> C
    C --> A: {{ iteration < 3 }}
    C --> D
    D --> [*]
```

> 更多详情参见 [流水线核心](workflow.md) 中循环图部分

---

## 9. Status（运行时状态）

| 字段 | 类型 | 说明 |
|------|------|------|
| `Status` | map | 节点运行时状态，键为节点名，值为状态枚举 |

### 状态枚举

| 值 | 含义 |
|-----|------|
| `Pending` | 等待执行 |
| `Running` | 执行中 |
| `SUCCESS` | 执行成功 |
| `FAILED` | 执行失败 |
| `CANCELLED` | 已取消 |
| `PAUSED` | 已暂停 |
| `STOPPED` | 已停止 |
| `ABORTED` | 已终止 |

---

## 10. Nodes（节点配置）

| 字段 | 类型 | 说明 |
|------|------|------|
| `Nodes.{name}.name` | string | 节点显示名称 |
| `Nodes.{name}.description` | string | 节点功能描述 |
| `Nodes.{name}.executor` | string | 引用 `Executors` 中的执行器名称 |
| `Nodes.{name}.image` | string | 容器镜像（Docker/K8s 执行器使用） |
| `Nodes.{name}.steps` | []Step | 执行步骤列表 |
| `Nodes.{name}.extract` | object | 输出提取配置（可选） |
| `Nodes.{name}.paramDelivery` | string | 参数传递方式：`env`（默认）、`args`、`stdin` |

### Step 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `steps[].id` | string | 步骤唯一标识（由 EnsureIds() 自动生成） |
| `steps[].name` | string | 步骤名称 |
| `steps[].description` | string | 步骤描述 |
| `steps[].run` | string | 执行的 shell 命令（支持模板渲染） |

### 参数传递

节点执行时可以通过三种方式接收参数：

| 方式 | 说明 | 配置示例 |
|------|------|----------|
| `env` | 参数作为环境变量注入（默认） | `paramDelivery: env` |
| `args` | 参数作为命令行参数 `--key=value` 传递 | `paramDelivery: args` |
| `stdin` | 参数序列化为 JSON 写入标准输入 | `paramDelivery: stdin` |

### 节点运行时状态（恢复执行）

节点配置支持 `runtime` 字段用于快照恢复：

```yaml
Nodes:
  Build:
    executor: local
    runtime:                    # 运行时状态（用于恢复）
      status: "SUCCESS"         # 已完成的节点会被跳过
      startTime: "2026-03-30T10:00:00Z"
      endTime: "2026-03-30T10:01:00Z"
      steps:
        - name: build
          status: "SUCCESS"
          output: "Build completed"
    steps:
      - name: build
        run: echo "Building..."
```

### 输出提取配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `extract.type` | string | `"codec-block"` | 提取类型：`codec-block` 或 `regex` |
| `extract.patterns` | map | - | 正则表达式（type=regex 时使用） |
| `extract.maxOutputSize` | int | 1048576 (1MB) | 输出大小限制（字节） |

> 更多详情参见 [节点与步骤](node.md)

---

## 11. flowx.json 节点注册（Studio 扩展）

FlowX Studio 支持通过 `flowx.json` 文件注册节点到节点注册中心。每个 `flowx.json` 可以声明一个或多个节点，包含完整的执行元数据。

### 单节点配置

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

### 多节点配置（数组）

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

### flowx.json 字段说明

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

### 与 YAML 配置的关系

`flowx.json` 中的 `executor` 配置与 YAML 中的 `Executors` 配置是互补的：
- `flowx.json` 描述节点本身的执行元数据（入口、镜像、语言等）
- YAML 中的 `Executors` 描述运行时的执行环境配置（网络、卷挂载、资源限制等）

```yaml
# flowx.json 注册节点后，在 YAML 中引用
Executors:
  image-resizer:
    type: local
    config:
      shell: bash

Nodes:
  Resize:
    executor: image-resizer
    steps:
      - name: resize
        run: python main.py
```

---

## 字段引用关系

```
Param ──┬──► Metadate.data（模板渲染）
        ├──► Nodes.steps[].run（命令参数渲染）
        ├──► Edge 表达式（条件求值）
        └──► Logging.headers（动态认证）

Executors ──► Nodes.executor（执行器选择）

Graph ──► 定义节点执行顺序和依赖关系

Status ──► 控制节点是否跳过（恢复执行）
```
