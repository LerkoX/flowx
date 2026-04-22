# Metadata 模块流程图

## MetadataStore 接口

```mermaid
flowchart TD
    A[MetadataStore] --> B[Get<br/>ctx, key → value<br/>获取元数据]
    A --> C[Set<br/>ctx, key, value<br/>设置元数据]
    A --> D[Delete<br/>ctx, key<br/>删除元数据]
    A --> E[Close<br/>关闭连接]
```

## MetadataStoreFactory 工厂

```mermaid
flowchart TD
    A[MetadataStoreFactory] --> B[Create<br/>config → MetadataStore]

    B --> C{MetadataConfig.Type}
    C -->|"http"| D[创建HTTP Store]
    C -->|"redis"| E[创建Redis Store]
    C -->|"in-config"| F[创建InConfig Store]
```

## InConfigMetadataStore 内存存储

```mermaid
flowchart TD
    subgraph internal_structure["内部结构"]
    A[InConfigMetadataStore] --> B[data<br/>map string string<br/>内存存储]
    end

    subgraph get_flow["Get流程"]
    C[Get] --> D{查找key}
    D -->|找到| E[返回值]
    D -->|未找到| F[return空字符串<br/>err]
    end

    subgraph set_flow["Set流程"]
    G[Set] --> H[data[key] = value]
    H --> I[return nil]
    end

    subgraph delete_flow["Delete流程"]
    J[Delete] --> K{delete data, key}
    K --> L[return nil]
    end

    subgraph get_all["GetAll"]
    M[GetAll] --> N[返回data副本<br/>map string string]
    end
```

## 元数据访问上下文

```mermaid
flowchart TD
    A[渲染上下文] --> B[Metadata<br/>map string any]
    A --> C[Param<br/>map string any]
    A --> D[nodeID.key<br/>平铺访问]

    B --> E[从MetadataStore加载]
    C --> F[用户传入参数]
    D --> G[节点提取结果]

    subgraph access_methods["访问方式"]
    H[双花括号Metadata.user]
    I[双花括号Param.name]
    J[双花括号Node1.result]
    end
```

## 元数据存储选择

```mermaid
flowchart TD
    A[选择存储类型] --> B{MetadataConfig.Type}

    B -->|"无配置或in-config"| C[InConfigMetadataStore]
    B -->|"redis"| D[RedisMetadataStore]
    B -->|"http"| E[HTTPMetadataStore]

    C --> F[进程内内存<br/>快速访问]
    D --> G[分布式Redis<br/>多进程共享]
    E --> H[远程HTTP API<br/>自定义后端]
```