# CLAUDE.md

Please answer in Chinese.

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based CI/CD pipeline execution library that supports multiple execution backends (Docker, Kubernetes, SSH, Local). The library uses a DAG (Directed Acyclic Graph) structure to manage pipeline dependencies and supports concurrent execution of independent tasks.

## Development Commands

### Building
```bash
go build ./...          # Build all packages in the project
go mod tidy             # Clean up dependencies
```

### Testing
```bash
go test ./test/         # Run all tests
go test ./test/ -v      # Run tests with verbose output
```

**Note**: All tests should pass. Run `go build ./...` before testing to ensure no compilation errors.

**Test Configuration File Maintenance**:
- Test configuration files are located in `test/fixtures/runtime/` directory
- The mapping between configuration files and test cases is documented in `test/fixtures/runtime/README.md`
- When adding new test cases, update `test/fixtures/runtime/README.md` to document the new configuration files

### Code Quality
```bash
go fmt ./...            # Format all Go code
go vet ./...            # Run static analysis
```

### Code Standards

**文件行数限制**：
- **每个 Go 文件不能超过 1000 行**
- 如果文件接近或超过 1000 行，应按功能拆分为多个文件
- 拆分原则：
  - 按功能分组（如：图结构、执行流程、配置解析）
  - 保持包名一致
  - 接口定义单独存放

## Architecture Overview

### Core Components

1. **Pipeline Interface** (`pipeline.go`): Main pipeline lifecycle management
2. **DAG Graph Implementation** (`pipeline_impl.go`): Manages task dependencies and traversal
3. **Node System** (`node.go`, `node_impl.go`): Individual task management with state tracking
4. **Executor Pattern** (`executor.go`): Pluggable backend execution system
5. **Runtime** (`runtime.go`, `runtime_impl.go`): Pipeline execution runtime with process safety
6. **Edge System** (`edge.go`, `edge_impl.go`): DAG edges with conditional expression support
7. **Template Engine** (`templete.go`, `templete_impl.go`): Template rendering for dynamic configuration
8. **Metadata Store** (`metadata.go`, `metadata_impl.go`): Process-safe metadata management
9. **Configuration** (`config.go`): Pipeline configuration structures and parsing

### Key Architecture Patterns

- **DAG-based Pipeline**: Tasks are nodes in a directed acyclic graph with dependencies
- **Executor Pattern**: Different execution backends (Function, Docker, K8s, SSH, Local)
- **Event-driven**: Pipeline and node lifecycle events for monitoring
- **Concurrent Execution**: Independent tasks run in parallel using goroutines
- **Conditional Edges**: Edges support conditional expressions for dynamic execution paths
- **Template Engine**: Support for template rendering in configuration
- **Metadata Management**: Process-safe metadata storage and retrieval during execution

### Directory Structure

- `/` - Core pipeline interfaces and implementations
- `/executor/` - Execution backend implementations
  - `/kubenetes/` - Fully implemented Kubernetes executor
  - `/docker/` - Placeholder for Docker executor
  - `/ssh/` - Placeholder for SSH executor
  - `/local/` - Placeholder for Local executor
- `/test/` - Test suite
- `/doc/` - Documentation files
  - `config.md` - Configuration detailed documentation
  - `templete.md` - Template engine documentation

### Configuration

Pipeline configuration uses YAML format with the following structure:
```yaml
Version: "1.0"           # Configuration version
Name: my-pipeline        # Pipeline name

Metadate:                # Metadata configuration
  type: in-config        # Metadata store type (in-config, redis, http)
  data:                  # Initial metadata key-value pairs
    key1: value1

AI:                      # AI-related configuration
  intent: "描述Pipeline意图"
  constraints:           # Key constraints
    - "约束1"
  template: "template-id"

Param:                   # Pipeline parameters
  key: value

Executors:               # Global executor definitions
  local:
    type: local
    config: {}
  docker:
    type: docker
    config: {}

Logging:                 # Log pushing configuration
  endpoint: http://log-center/api/v1/logs
  headers: {}
  timeout: 5s

Graph: |                 # DAG definition (Mermaid stateDiagram-v2 format)
  stateDiagram-v2
    [*] --> Node1
    Node1 --> Node2

Status:                  # Node runtime status
  Node1: Finished

Nodes:                   # Node definitions with execution details
  NodeName:
    executor: local      # Reference to global executor
    image: optional-image
    steps:               # Multi-step execution
      - name: step1
        run: command
```

See `config.example.yaml` for a complete example.

## Current Implementation Status

**Implemented**:
- ✅ Core pipeline DAG structure and traversal
- ✅ Basic pipeline execution with concurrent processing
- ✅ Kubernetes executor (fully functional)
- ✅ Node state management and event system
- ✅ Cycle detection and graph validation
- ✅ Conditional edges with expression evaluation
- ✅ Template engine for dynamic configuration
- ✅ Metadata store interface and implementations
- ✅ Pipeline Runtime with process safety
- ✅ Multi-step node execution
- ✅ Graph text visualization
- ✅ Log pushing interface

**Planned but not implemented**:
- 🔄 Function executor
- 🔄 Docker executor
- 🔄 SSH executor
- 🔄 Local executor

## Development Notes

- This is a library, not an executable application (no main.go)
- Uses Go 1.23.0 with heavy Kubernetes integration
- The project follows clean architecture with good separation of concerns
- Main development branch is `master`

## Working with the Codebase

When making changes:
1. Understand the DAG traversal algorithm in `pipeline_impl.go`
2. Check the executor interfaces in `executor.go` before implementing new backends
3. Follow the event-driven pattern for pipeline monitoring
4. Ensure graph validation and cycle detection are maintained
5. Run `go build ./...` frequently to catch compilation issues early
6. For conditional logic, check `edge.go` and `eval_context.go`
7. For runtime features, check `runtime.go` and `runtime_impl.go`
8. For template functionality, check `templete.go` and related tests


## Issue Archive

### 2026-04-16: Pipeline 输出未通过 Pusher 推送

**问题描述**：
- `wait_input_example.yaml` 工作流执行成功，但脚本中的 `echo` 语句输出没有通过日志推送器（logger.Pusher）推送
- 输出通过 `fmt.Print` 直接打印到标准输出
- 用户反馈："How could it output here, isn't there a log adapter that can output?"

**根本原因**：
- `RuntimeImpl` 虽然有 `pusher` 字段，但从未实际使用
- `PipelineImpl` 没有 `pusher` 字段，无法在内部推送日志
- `resultChan` 已包含实时输出，但 `handleResult` 方法只调用 `fmt.Print`

**解决方案**：
1. 给 `PipelineImpl` 添加 `pusher logger.Pusher` 字段
2. 在 `Pipeline` 接口中添加 `SetPusher` 方法
3. `RuntimeImpl` 创建 Pipeline 时传递 `pusher`
4. `handleResult` 中删除 `fmt.Print`，改用 `pusher.Push` 推送日志

**影响范围**：
- `dag/pipeline.go`：接口添加
- `dag/p`ipeline_impl.go`：结构体和实现修改
- `runtime_impl.go`：创建 Pipeline 时传递 pusher
- `test/pipeline_output_test.go`：新增测试用例

**验证**：
- 单元测试：`TestOutputPusher` 验证输出通过 pusher 推送
- 集成测试：`wait_input_example.yaml` 验证 echo 语句正确显示
