# FlowX - 项目概述

## 简介

FlowX（包名 `github.com/LerkoX/flowx`）是一个基于 Go 语言开发的 CI/CD 流水线执行库。它使用 DAG（有向无环图）结构管理任务依赖关系，支持独立任务的并发执行，并提供可插拔的执行后端（Local、Docker、Kubernetes）。

核心特性：

- **DAG 流水线**：任务以有向无环图形式组织，支持复杂的依赖关系
- **循环图支持**：条件回边产生可控循环，通过 `iteration` 变量控制迭代次数
- **并发执行**：无依赖关系的节点自动并行运行
- **多执行后端**：支持 Local（本地 Shell）、Docker（容器）、Kubernetes（Pod）三种执行环境
- **条件边**：支持基于表达式的条件分支，动态控制执行路径
- **模板引擎**：基于 pongo2 的模板渲染，支持参数引用和自引用
- **元数据管理**：进程安全的元数据存储，支持 in-config、Redis、HTTP 三种后端
- **输出提取**：从命令输出中提取结构化数据，供后续节点使用
- **快照恢复**：支持导出运行状态并从检查点恢复执行
- **事件驱动**：完整的流水线和节点生命周期事件系统
- **暂停恢复**：流水线可暂停/恢复，暂停期间支持动态修改图结构
- **动态图修改**：运行中可安全添加/删除节点和边

## 架构总览

```
┌─────────────────────────────────────────────────────────┐
│                      Runtime（运行时）                     │
│  - 同步/异步执行                                          │
│  - 流水线生命周期管理                                      │
│  - 快照与恢复                                             │
├─────────────────────────────────────────────────────────┤
│                   Pipeline（流水线）                       │
│  - DAG 图遍历（BFS，统一支持有环/无环）                      │
│  - 循环执行（条件回边 + iteration 计数器）                    │
│  - 暂停/恢复                                              │
│  - 并发节点执行                                           │
│  - 事件通知                                               │
│  - 模板渲染                                               │
├───────────────┬───────────────┬─────────────────────────┤
│     Node      │     Edge      │    MetadataStore        │
│  节点与步骤    │  条件边        │    元数据存储             │
│  状态管理      │  表达式求值     │    in-config/redis/http │
│  输出提取      │               │                         │
├───────────────┴───────────────┴─────────────────────────┤
│                  Executor（执行器）                        │
│  ┌──────────┐  ┌──────────┐  ┌────────────────────┐    │
│  │  Local   │  │  Docker  │  │    Kubernetes       │    │
│  │ 本地Shell │  │ 容器执行  │  │    Pod 执行         │    │
│  └──────────┘  └──────────┘  └────────────────────┘    │
├─────────────────────────────────────────────────────────┤
│              Template Engine（模板引擎）                    │
│              基于 pongo2，支持条件、循环、过滤器              │
├─────────────────────────────────────────────────────────┤
│                 Logger（日志系统）                          │
│           日志推送接口，支持自定义 Pusher                    │
└─────────────────────────────────────────────────────────┘
```

## 目录结构

```
flowx/
├── core/                 # 核心类型定义
│   ├── config.go         # 配置结构体定义
│   ├── const.go          # 常量（状态、事件）
│   ├── field.go          # FieldItem 定义
│   └── uuid.go           # UUID 工具函数
├── dag/                  # DAG 流水线核心
│   ├── pipeline.go       # Pipeline 接口定义
│   ├── pipeline_impl.go  # Pipeline + DAG 图实现
│   ├── node.go           # Node 接口定义
│   ├── node_impl.go      # Node 实现
│   ├── edge.go           # Edge 接口定义
│   ├── edge_impl.go      # Edge 实现（条件边）
│   ├── eval_context.go   # 表达式求值上下文
│   ├── extractor.go      # 输出提取器
│   └── snapshot.go       # 快照与恢复
├── executor/             # 执行器系统
│   ├── interfaces.go     # Executor/Adapter/Bridge 接口
│   ├── provider/
│   │   └── provider.go   # Executor 提供者（工厂模式）
│   ├── docker/           # Docker 执行器
│   ├── local/            # Local 执行器
│   └── kubernetes/       # Kubernetes 执行器
├── template/             # 模板引擎
│   ├── template.go       # 模板引擎接口
│   └── template_impl.go  # pongo2 模板引擎实现
├── metadata/             # 元数据存储
│   ├── metadata.go       # MetadataStore 接口
│   └── metadata_impl.go  # 三种存储实现
├── logger/               # 日志系统
│   ├── logger.go         # 日志接口与类型
│   └── console_pusher.go # 控制台日志推送
├── doc/                  # 文档目录
├── test/                 # 测试
└── config.example.yaml   # 示例配置文件
```

## 快速开始

### 安装

```bash
go get github.com/LerkoX/flowx
```

### 基本使用

```go
package main

import (
    "context"
    "fmt"
    "github.com/LerkoX/flowx"
)

func main() {
    // 1. 创建运行时
    rt := flowx.NewRuntime(context.Background())

    // 2. 准备 YAML 配置
    configYAML := `
Version: "1.0"
Name: hello-pipeline

Executors:
  local:
    type: local
    config:
      shell: bash

Graph: |
  stateDiagram-v2
    [*] --> Hello
    Hello --> [*]

Nodes:
  Hello:
    executor: local
    steps:
      - name: greet
        run: echo "Hello, Pipelinex!"
`

    // 3. 同步执行
    p, err := rt.RunSync(context.Background(), "hello-001", configYAML, nil)
    if err != nil {
        panic(err)
    }

    // 4. 等待完成
    <-p.Done()
    fmt.Println("Pipeline status:", p.Status())
}
```

### 带事件监听

```go
// 实现 Listener 接口
type MyListener struct{}

func (l *MyListener) Events() []flowx.Event {
    return []flowx.Event{
        flowx.EventPipelineNodeStart,
        flowx.EventPipelineNodeFinish,
    }
}

func (l *MyListener) Handle(p flowx.Pipeline, event flowx.Event) {
    fmt.Printf("Event: %s, Pipeline: %s\n", event, p.Id())
}

listener := &MyListener{}
p, _ := rt.RunSync(ctx, "pipeline-001", configYAML, listener)
```

## 相关文档

- [配置参考](configuration.md) - 完整 YAML 配置字段说明
- [流水线核心](pipeline.md) - DAG 图结构与生命周期
- [节点与步骤](node.md) - 节点配置与多步骤执行
- [执行器系统](executor.md) - Local/Docker/Kubernetes 执行器
- [条件边](edge.md) - 条件表达式与分支控制
- [模板引擎](template.md) - 模板渲染机制
- [元数据存储](metadata.md) - 元数据管理
- [运行时管理](runtime.md) - Runtime 接口与快照恢复
