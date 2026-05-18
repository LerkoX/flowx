# 元数据存储

本文档介绍元数据存储（MetadataStore）接口及其三种实现。

## MetadataStore 接口

```go
type MetadataStore interface {
    Get(ctx context.Context, key string) (string, error)    // 获取值
    Set(ctx context.Context, key, value string) error       // 设置值
    Delete(ctx context.Context, key string) error           // 删除值
    Close() error                                           // 关闭连接
}
```

## MetadataStoreFactory 接口

```go
type MetadataStoreFactory interface {
    Create(config MetadataConfig, pipelineId string) (MetadataStore, error)    // 根据配置创建存储，pipelineId 用于数据隔离
}
```

默认实现：`DefaultMetadataStoreFactory`，根据 `MetadataConfig.Type` 选择后端。

**pipelineId 隔离机制：**
- **HTTP**：通过 `X-Pipeline-ID` 请求头传递
- **Redis**：Key 前缀为 `flowx/{pipelineId}/{key}`
- **in-config**：不依赖 pipelineId

## InConfigMetadataStore

内存存储，基于 `sync.RWMutex` 的线程安全 map。

### 配置

```yaml
Metadate:
  type: in-config
  data:
    key1: value1
    key2: value2
```

### 特点

- 初始数据从配置的 `data` 字段加载
- 纯内存，无外部依赖
- 适合单次执行、测试场景
- `Close()` 为空操作

### 使用

```go
factory := NewMetadataStoreFactory()
store, _ := factory.Create(MetadataConfig{
    Type: "in-config",
    Data: map[string]any{"key1": "value1"},
}, "pipeline-001")

store.Set(ctx, "key2", "value2")
val, _ := store.Get(ctx, "key1")  // "value1"
store.Delete(ctx, "key1")
store.Close()
```

## HTTPMetadataStore

通过 HTTP API 进行元数据的读写。

### 配置

```yaml
Metadate:
  type: http
  data:
    url: "http://metadata-service/api/v1/data"
    method: "GET"        # Get 方法（仅影响 Get 请求）
    headers:
      Authorization: "Bearer token123"
    timeout: "5s"
```

### 配置项

| 键 | 类型 | 说明 |
|-----|------|------|
| `url` | string | HTTP 服务地址（必填） |
| `method` | string | Get 请求方法（默认 GET），Set/Delete 固定使用 POST/DELETE |
| `headers` | map | 自定义请求头 |
| `timeout` | string | 超时时间 |

### API 约定

- **Get**：发送 `{method}` 请求到 `{url}?key={key}`（key 作为查询参数）
- **Set**：发送 `POST` 请求到 `{url}`，body 为 JSON `{"key": "...", "value": "..."}`
- **Delete**：发送 `DELETE` 请求到 `{url}?key={key}`

### 请求头

除自定义 headers 外，HTTP Store 会自动设置以下请求头：
- `Content-Type: application/json`
- `X-Pipeline-ID: {pipelineId}`（当 pipelineId 不为空时）

### 特点

- 适合与外部元数据服务集成
- 支持自定义认证头
- 可配置超时

## RedisMetadataStore

基于 Redis 的键值存储。

### 配置

```yaml
Metadate:
  type: redis
  data:
    host: "localhost"
    port: "6379"
    db: 0
    username: ""
    password: ""
```

### 配置项

| 键 | 类型 | 说明 |
|-----|------|------|
| `host` | string | Redis 地址 |
| `port` | string | Redis 端口 |
| `db` | int | 数据库编号 |
| `username` | string | 用户名（可选） |
| `password` | string | 密码（可选） |

### 特点

- 使用 `go-redis/v9` 客户端
- 支持持久化存储
- **Pipeline 隔离**：不同流水线的数据通过 key 前缀隔离，格式为 `flowx/{pipelineId}/{key}`
- 适合分布式场景
- `Close()` 关闭 Redis 连接

### Pipeline 隔离示例

```yaml
# pipeline-001 写入 key="mykey"
# 实际存储的 Redis key: flowx/pipeline-001/mykey

# pipeline-002 写入同名 key="mykey"
# 实际存储的 Redis key: flowx/pipeline-002/mykey
# 两者互不干扰
```

## 数据流

元数据在流水线中的流转：

```
1. 初始化：从配置的 Metadate.data 加载初始数据（srcNode=""）
      ↓
2. 模板渲染：Metadata.data 中的 {{ Param.xxx }} 被渲染
      ↓
3. 节点执行：步骤命令可以引用 Metadata 中的值
      ↓
4. 输出提取：节点执行完成后，extractor 提取输出中的数据
      ↓
5. 写入 Metadata：提取的数据写入 MetadataStore（自动设置 srcNode=节点ID）
      ↓
6. 下游节点：后续节点可以引用新写入的 Metadata
```

## SrcNode 追踪

从节点输出提取的元数据会自动记录来源节点：

- **配置初始化**：srcNode 为空字符串（`""`）
- **节点产生**：srcNode 自动设置为产生该数据的节点 ID

存储格式：`{nodeId}.{key}` → FieldItem

例如：`build-node.imageTag` 表示由 `build-node` 节点产生

### FieldItem 结构

```go
type FieldItem struct {
    Value       interface{} // 字段值
    Description string     // 字段描述（从 flowx-yaml 注释提取）
    SrcNode     string     // 来源节点 ID
}
```

### 示例

```yaml
Metadate:
  type: in-config
  data:
    buildVersion: "{{ Param.version }}"  # srcNode=""

Nodes:
  Build:
    executor: docker
    extract:
      type: codec-block
    steps:
      - name: build
        run: |
          echo '```flowx-yaml'
          echo "imageTag: \"myapp:{{ Param.version }}\"  # 镜像标签"
          echo "buildStatus: \"success\"  # 构建状态"
          echo '```'
          # 提取后:
          # Metadata["Build.imageTag"] = FieldItem{Value: "myapp:v1.0.0", Description: "镜像标签", SrcNode: "Build"}
          # Metadata["Build.buildStatus"] = FieldItem{Value: "success", Description: "构建状态", SrcNode: "Build"}

  Deploy:
    executor: k8s
    steps:
      - name: deploy
        run: |
          echo "Deploying {{ Metadata.Build.imageTag }}"
          echo "Status: {{ Metadata.Build.buildStatus }}"
```

## MetadataConfig 结构体

```go
type MetadataConfig struct {
    Type        string         // "in-config" | "http" | "redis"
    Description string         // 描述
    Data        map[string]any // 初始数据 / 后端配置
}
```

## InConfigMetadataStore 扩展方法

`InConfigMetadataStore` 除接口方法外，还提供以下便捷方法：

```go
func (s *InConfigMetadataStore) GetAll() map[string]string  // 获取所有元数据的副本
func (s *InConfigMetadataStore) Keys() []string             // 获取所有键
```

## 线程安全

所有三种实现都保证并发安全：

- **InConfig**：使用 `sync.RWMutex` 保护内部 map
- **HTTP**：依赖 HTTP 服务端实现并发控制
- **Redis**：依赖 Redis 服务端实现并发控制
- 多个 Pipeline 可以安全地并发访问同一个 Store 实例
