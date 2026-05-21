# FlowX Web UI + AI 集成 技术设计文档

## 文档索引

本文档集详细描述了 FlowX 项目从纯命令行工作流引擎向**AI 驱动的可视化工作流平台**演进的完整技术设计方案。

| 章节 | 文件 | 内容 |
|------|------|------|
| 1 | [01-overview.md](./01-overview.md) | 项目概述、愿景与核心原则 |
| 2 | [02-architecture.md](./02-architecture.md) | 系统整体架构、模块划分与交互关系 |
| 3 | [03-database.md](./03-database.md) | SQLite 数据库 Schema 设计 |
| 4 | [04-api.md](./04-api.md) | REST API 接口设计规范 |
| 5 | [05-frontend.md](./05-frontend.md) | React 前端架构、组件设计与状态管理 |
| 6 | [06-ai-service.md](./06-ai-service.md) | AI 服务层设计、多模型支持与 Prompt 工程 |
| 7 | [07-node-system.md](./07-node-system.md) | 节点注册中心、Mock 模式与执行引擎 |
| 8 | [08-runtime.md](./08-runtime.md) | 运行时架构、单二进制部署与进程管理 |
| 9 | [09-security.md](./09-security.md) | 安全设计、错误处理与日志规范 |

## 设计原则

1. **零人工配置**：用户仅通过自然语言描述需求，AI 完成所有实现和编排
2. **单二进制部署**：`flowx` 一个命令启动完整平台
3. **本地化优先**：数据存储在本地 SQLite，支持离线运行
4. **引擎复用**：最大化复用现有 FlowX DAG 引擎能力
5. **多模型兼容**：支持 OpenAI、Anthropic、Ollama 等多种 AI 提供商
