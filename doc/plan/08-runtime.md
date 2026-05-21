# 8. 运行时与部署设计

## 8.1 单二进制架构

### 8.1.1 设计目标

- **单个文件**：`flowx` 一个二进制文件包含所有功能
- **零依赖**：无需安装 Node.js、Python 等运行时
- **自包含**：前端资源嵌入二进制，无需外部文件
- **快速启动**：3 秒内完成启动并打开浏览器

### 8.1.2 构建策略

```
┌─────────────────────────────────────────────────────────────┐
│                    flowx binary                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  Go 编译的 HTTP 服务器 + 业务逻辑                      │  │
│  │  ┌───────────────────────────────────────────────┐   │  │
│  │  │  前端静态资源 (go:embed web/dist/*)            │   │  │
│  │  │  - index.html                                  │   │  │
│  │  │  - *.js, *.css                                 │   │  │
│  │  │  - assets/                                     │   │  │
│  │  └───────────────────────────────────────────────┘   │  │
│  │  ┌───────────────────────────────────────────────┐   │  │
│  │  │  后端代码                                      │   │  │
│  │  │  - HTTP Server                                 │   │  │
│  │  │  - Business Logic                              │   │  │
│  │  │  - AI Service                                  │   │  │
│  │  │  - DB Layer (SQLite)                           │   │  │
│  │  └───────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  现有 FlowX 引擎 (dag, executor, template, etc.)       │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 8.1.3 go:embed 嵌入前端资源

```go
package server

import (
    "embed"
    "io/fs"
    "net/http"
)

//go:embed all:web/dist
var webDist embed.FS

func NewStaticHandler() http.Handler {
    // 从嵌入的文件系统创建 http.FS
    distFS, err := fs.Sub(webDist, "web/dist")
    if err != nil {
        panic(err)
    }
    
    fileServer := http.FileServer(http.FS(distFS))
    
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // SPA 路由处理：所有非 API 请求返回 index.html
        if !strings.HasPrefix(r.URL.Path, "/api/") {
            // 检查文件是否存在
            _, err := distFS.Open(strings.TrimPrefix(r.URL.Path, "/"))
            if err != nil {
                // 文件不存在，返回 index.html（SPA 路由）
                r.URL.Path = "/"
            }
        }
        fileServer.ServeHTTP(w, r)
    })
}
```

### 8.1.4 构建流程

```bash
# 1. 构建前端
cd web
npm install
npm run build

# 2. 构建后端（嵌入前端）
cd ..
go build -o flowx cmd/flowx/main.go

# 最终产物：单个 flowx 二进制文件
```

## 8.2 命令行接口

### 8.2.1 命令设计

```bash
# 启动 Web UI（默认命令）
flowx
flowx server

# 运行 YAML 工作流（保持现有功能）
flowx run workflow.yaml
flowx run workflow.yaml --param key=value

# 查看版本
flowx version

# 查看帮助
flowx help
flowx help server
flowx help run
```

### 8.2.2 启动参数

```bash
flowx server [flags]

Flags:
  -p, --port int        HTTP 服务端口 (默认 8080)
  -H, --host string     监听地址 (默认 "0.0.0.0")
      --no-open         不自动打开浏览器
      --data-dir string 数据目录 (默认 "~/.flowx")
      --debug           启用调试模式
```

### 8.2.3 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `FLOWX_PORT` | HTTP 服务端口 | 8080 |
| `FLOWX_HOST` | 监听地址 | 0.0.0.0 |
| `FLOWX_DATA_DIR` | 数据目录 | ~/.flowx |
| `FLOWX_DB_PATH` | 数据库文件路径 | ~/.flowx/flowx.db |
| `FLOWX_NO_OPEN` | 不自动打开浏览器 | false |
| `FLOWX_DEBUG` | 调试模式 | false |
| `FLOWX_ENCRYPTION_KEY` | API Key 加密密钥 | 自动生成 |

## 8.3 进程管理

### 8.3.1 启动流程

```
用户执行 flowx
    │
    ▼
[命令解析] 识别为 server 命令
    │
    ▼
[初始化] 
  ├── 创建数据目录 ~/.flowx
  ├── 初始化数据库（如有必要）
  ├── 加载系统配置
  └── 检查 AI 配置
    │
    ▼
[启动 HTTP 服务器]
  ├── 注册 API 路由
  ├── 注册静态资源路由
  └── 注册 SSE 路由
    │
    ▼
[打开浏览器]（如果未禁用）
    │
    ▼
[等待请求]
```

### 8.3.2 优雅关闭

```go
func main() {
    // 创建服务器
    server := NewServer(config)
    
    // 启动服务器（在 goroutine 中）
    go server.Start()
    
    // 监听系统信号
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    
    // 等待信号
    <-sigCh
    
    // 优雅关闭
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        log.Printf("Server shutdown error: %v", err)
    }
    
    // 关闭数据库连接
    db.Close()
    
    log.Println("Server stopped")
}
```

### 8.3.3 单实例机制

防止同时运行多个实例导致端口冲突：

```go
func ensureSingleInstance(dataDir string) (func(), error) {
    pidFile := filepath.Join(dataDir, "flowx.pid")
    
    // 检查是否已有实例在运行
    if data, err := os.ReadFile(pidFile); err == nil {
        pid := string(data)
        if isProcessRunning(pid) {
            return nil, fmt.Errorf("FlowX is already running (PID: %s)", pid)
        }
    }
    
    // 写入当前 PID
    if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
        return nil, err
    }
    
    // 返回清理函数
    return func() {
        os.Remove(pidFile)
    }, nil
}
```

## 8.4 数据目录结构

```
~/.flowx/
├── flowx.db              # SQLite 数据库
├── flowx.db.backup       # 自动备份
├── flowx.pid             # 进程 ID 文件
├── logs/
│   ├── flowx.log         # 应用日志
│   └── executions/       # 执行日志（可选，大量时分离）
├── temp/
│   └── mock-*/           # Mock 测试临时文件
├── nodes/
│   └── {node_name}/      # 节点代码缓存
│       ├── main.py
│       ├── requirements.txt
│       └── Dockerfile
└── config.yaml           # 可选：外部配置文件
```

## 8.5 自动浏览器打开

### 8.5.1 实现机制

```go
func openBrowser(url string) error {
    var cmd string
    var args []string
    
    switch runtime.GOOS {
    case "darwin":
        cmd = "open"
        args = []string{url}
    case "windows":
        cmd = "rundll32"
        args = []string{"url.dll,FileProtocolHandler", url}
    default: // linux
        cmd = "xdg-open"
        args = []string{url}
    }
    
    return exec.Command(cmd, args...).Start()
}
```

### 8.5.2 启动时机

- 服务器成功启动后（端口监听就绪）
- 延迟 500ms 确保服务完全初始化
- 仅在桌面环境自动打开（检测 `DISPLAY` 环境变量等）
- 可通过 `--no-open` 或 `FLOWX_NO_OPEN=true` 禁用

## 8.6 日志系统

### 8.6.1 日志分级

| 级别 | 用途 | 输出位置 |
|------|------|---------|
| DEBUG | 开发调试信息 | 控制台（debug 模式） |
| INFO | 正常运行信息 | 控制台 + 文件 |
| WARN | 警告信息 | 控制台 + 文件 |
| ERROR | 错误信息 | 控制台 + 文件 |
| FATAL | 致命错误 | 控制台 + 文件，程序退出 |

### 8.6.2 日志格式

```
2025-01-20T10:00:00.123+0800 [INFO] [server] HTTP server started on :8080
2025-01-20T10:00:01.456+0800 [INFO] [browser] Opened browser at http://localhost:8080
2025-01-20T10:05:00.789+0800 [INFO] [execution] Execution 42 started for workflow 8
2025-01-20T10:05:02.012+0800 [INFO] [node] Node 'download' started
2025-01-20T10:05:32.345+0800 [INFO] [node] Node 'download' completed in 30.333s
```

### 8.6.3 日志轮转

- 日志文件最大 10MB
- 保留最近 5 个日志文件
- 使用 lumberjack 库实现

## 8.7 现有引擎改进计划

### 8.7.1 运行时状态查询接口

现有 FlowX 引擎在执行时不暴露内部状态。需要添加查询接口，支持 Web 层实时监控：

```go
// 在 Runtime 接口中新增
 type Runtime interface {
     // 现有方法...
     Run(ctx context.Context, config *core.PipelineConfig) error
     
     // 新增：状态查询
     GetExecutionState() (*ExecutionState, error)
     GetNodeState(nodeID string) (*NodeState, error)
     
     // 新增：事件订阅
     OnNodeStart(handler NodeStartHandler)
     OnNodeComplete(handler NodeCompleteHandler)
     OnLog(handler LogHandler)
 }
```

### 8.7.2 执行取消支持

增强引擎支持通过 context 取消执行：

```go
// 检查 context 取消信号
func (p *PipelineImpl) executeNode(ctx context.Context, node *dag.Node) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    
    // 执行节点...
}
```

### 8.7.3 日志流接口

为执行器添加日志流接口，支持实时捕获节点输出：

```go
// executor/interfaces.go
type LogHandler func(level string, message string)

type Executor interface {
    // 现有方法...
    
    // 新增：设置日志处理器
    SetLogHandler(handler LogHandler)
}
```

### 8.7.4 元数据存储改进

现有元数据存储支持 in-config、redis、http 三种后端。新增 SQLite 后端，与 Web 层共享数据库：

```go
// metadata/sqlite_store.go
type SQLiteStore struct {
    db *sql.DB
}

func (s *SQLiteStore) Get(key string) (string, error) {
    var value string
    err := s.db.QueryRow("SELECT value FROM metadata WHERE key = ?", key).Scan(&value)
    return value, err
}

func (s *SQLiteStore) Set(key string, value string) error {
    _, err := s.db.Exec(
        "INSERT INTO metadata (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?",
        key, value, value,
    )
    return err
}
```

## 8.8 配置加载优先级

配置来源按优先级排序（高优先级覆盖低优先级）：

1. **命令行参数**：`--port 9090`
2. **环境变量**：`FLOWX_PORT=9090`
3. **配置文件**：`~/.flowx/config.yaml`
4. **默认值**：`8080`

```yaml
# ~/.flowx/config.yaml 示例
server:
  port: 8080
  host: "0.0.0.0"
  auto_open_browser: true

data:
  dir: "~/.flowx"
  db_path: "~/.flowx/flowx.db"

ai:
  default_provider: "openai"
  providers:
    - name: "OpenAI"
      provider: "openai"
      model: "gpt-4"
      api_key: "${OPENAI_API_KEY}"  # 支持环境变量引用
      temperature: 0.7

execution:
  default_executor: "local"
  max_concurrent: 5
  timeout: "1h"

mock:
  timeout: "30s"
  use_docker: true
```
