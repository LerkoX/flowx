# Metadata 模块流程图

## 元数据存储流程

```mermaid
flowchart TD
    A[MetadataStore Interface] --> B[Get: 获取值]
    A --> C[Set: 设置值]
    A --> D[Delete: 删除值]
    A --> E[Close: 关闭连接]

    B --> F[key: 元数据键]
    F --> G[返回值或错误]

    C --> H[key: 元数据键]
    C --> I[value: 元数据值]
    H --> J[成功或错误]
```

## 元数据工厂流程

```mermaid
flowchart TD
    A[MetadataStoreFactory] --> B[Create: 创建存储实例]
    B --> C[MetadataConfig]
    
    C --> D{配置类型}
    D -->|http| E[HTTP元数据存储]
    D -->|redis| F[Redis元数据存储]
    D -->|in-config| G[内存元数据存储]
```

## In-Config元数据存储流程

```mermaid
flowchart TD
    A[InConfigMetadataStore] --> B[Get]
    B --> C{查找键]
    C -->|找到| D[返回值]
    C -->|未找到| E[返回错误]

    A --> F[Set]
    F --> G[保存到内存map]

    A --> H[Delete]
    H --> I[从内存map删除]
```