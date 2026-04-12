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
    Create(config MetadataConfig) (MetadataStore, error)    // 根据配置创建存储
}
```

默认实现：`DefaultMetadataStoreFactory`，根据 `MetadataConfig.Type` 选择后端。

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
factory := &DefaultMetadataStoreFactory{}
store, _ := factory.Create(MetadataConfig{
    Type: "in-config",
    Data: map[string]any{"key1": "value1"},
})

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
    endpoint: "http://metadata-service/api/v1/data"
    method: "GET"        # 读取方法
    headers:
      Authorization: "Bearer token123"
    timeout: "5s"
```

### 配置项

| 键 | 类型 | 说明 |
|-----|------|------|
| `endpoint` | string | HTTP 服务地址 |
| `method` | string | HTTP 方法（默认 GET） |
| `headers` | map | 请求头 |
| `timeout` | string | 超时时间 |

### API 约定

- **Get**：发送 `{method}` 请求到 `{endpoint}/{key}`
- **Set**：发送 `POST` 请求到 `{endpoint}/{key}`，body 为 value
- **Delete**：发送 `DELETE` 请求到 `{endpoint}/{key}`

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
- 适合分布式场景，多个 Pipeline 实例共享元数据
- `Close()` 关闭 Redis 连接

## 数据流

元数据在流水线中的流转：

```
1. 初始化：从配置的 Metadate.data 加载初始数据
      ↓
2. 模板渲染：Metadata.data 中的 {{ Param.xxx }} 被渲染
      ↓
3. 节点执行：步骤命令可以引用 Metadata 中的值
      ↓
4. 输出提取：节点执行完成后，extractor 提取输出中的数据
      ↓
5. 写入 Metadata：提取的数据写入 MetadataStore
      ↓
6. 下游节点：后续节点可以引用新写入的 Metadata
```

### 示例

```yaml
Metadate:
  type: in-config
  data:
    buildVersion: "{{ Param.version }}"

Nodes:
  Build:
    executor: docker
    extract:
      type: codec-block
    steps:
      - name: build
        run: |
          VERSION=$(cat version.txt)
          echo '```flowx-json'
          echo "{\"binaryName\": \"app\", \"binarySize\": \"$(stat -c%s app)\"}"
          echo '```'
          # 提取后: Metadata.binaryName = "app"
          #          Metadata.binarySize = "xxx"

  Deploy:
    executor: k8s
    steps:
      - name: deploy
        # 引用 Build 节点提取的数据
        run: |
          echo "Deploying {{ Metadata.binaryName }}"
          echo "Binary size: {{ Metadata.binarySize }}"
```

## MetadataConfig 结构体

```go
type MetadataConfig struct {
    Type        string         // "in-config" | "http" | "redis"
    Description string         // 描述
    Data        map[string]any // 初始数据 / 后端配置
}
```

## 线程安全

所有三种实现都使用 `sync.RWMutex` 保证并发安全：

- 读操作使用读锁（`RLock`）
- 写操作使用写锁（`Lock`）
- 多个 Pipeline 可以安全地并发访问同一个 Store 实例
