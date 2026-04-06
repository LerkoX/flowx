# 交互式输入功能 v2 - 程序主动请求输入

从 v2.0 开始，Pipeline 支持程序通过输出结构化 JSON 标记来主动请求用户输入。

## 功能说明

程序可以通过输出特殊格式的 JSON 来通知流水线："我需要用户输入"。

流水线检测到标记后会：
1. 自动将节点状态设为 `PAUSED`
2. 在 `NodeRuntimeStatus.InputRequest` 中存储输入请求信息
3. 等待外部程序通过 `InputChan` 发送输入

## JSON 标记格式

```json
{"pipelinex":"wait-input","prompt":"请输入用户名:","type":"text"}
```

**字段说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `pipelinex` | string | 是 | 固定值 `"wait-input"` |
| `prompt` | string | 否 | 显示给用户的提示信息 |
| `type` | string | 否 | 输入类型：`text`(默认)、`password`、`confirm` |
| `timeout` | int | 否 | 等待超时（秒），0表示无超时 |

## 使用示例

### 示例 1：Shell 脚本请求输入

```yaml
Nodes:
  InteractiveNode:
    executor: local
    steps:
      - name: ask-name
        run: |
          echo "开始交互流程..."
          echo '{"pipelinex":"wait-input","prompt":"请输入您的姓名:","type":"text"}'
          read name
          echo "你好，$name！"
```

### 示例 2：多步骤输入

```yaml
Nodes:
  MultiInputNode:
    executor: local
    steps:
      - name: setup
        run: |
          echo "配置向导"
          
          # 请求用户名
          echo '{"pipelinex":"wait-input","prompt":"用户名:","type":"text"}'
          read username
          
          # 请求密码
          echo '{"pipelinex":"wait-input","prompt":"密码:","type":"password"}'
          read password
          
          # 确认
          echo '{"pipelinex":"wait-input","prompt":"确认提交？(yes/no):","type":"confirm"}'
          read confirm
          
          if [ "$confirm" = "yes" ]; then
            echo "配置完成: $username"
          fi
```

## 外部程序处理流程

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/LerkoX/pipelinex"
)

func main() {
    ctx := context.Background()
    runtime := pipelinex.NewRuntime(ctx)

    // 运行流水线
    pipeline, _ := runtime.RunAsync(ctx, "demo", config, nil)

    // 启动输入处理 goroutine
    go handleInputRequests(ctx, pipeline)

    <-pipeline.Done()
}

func handleInputRequests(ctx context.Context, pipeline pipelinex.Pipeline) {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // 遍历所有节点
            for _, node := range pipeline.GetGraph().Nodes() {
                // 检测 PAUSED 状态（等待输入）
                if node.Status() == pipelinex.StatusPaused {
                    runtimeStatus := node.GetRuntimeStatus()
                    if runtimeStatus != nil && runtimeStatus.InputRequest != nil {
                        req := runtimeStatus.InputRequest

                        // 显示提示并获取输入
                        fmt.Printf("%s ", req.Prompt)
                        var input string
                        fmt.Scanln(&input)

                        // 发送输入
                        if runtimeStatus.InputChan != nil {
                            runtimeStatus.InputChan <- []byte(input + "\n")
                        }
                    }
                }
            }
        }
    }
}
```

## 状态流转

```
PENDING -> RUNNING -> PAUSED (请求输入) -> RUNNING (收到输入) -> SUCCESS/FAILED
```

- **PAUSED 状态**：表示程序正在等待用户输入
- **InputRequest**：包含 `StepName`、`Prompt`、`Type` 信息
- **恢复执行**：外部程序发送输入到 `InputChan` 后自动恢复

## 与旧版交互输入的区别

| 特性 | 旧版 (InputChan) | 新版 (Wait-Input 标记) |
|------|-----------------|---------------------|
| 触发方式 | 外部程序主动发送输入 | 程序主动请求输入 |
| 状态标识 | 无特定状态 | PAUSED 状态 |
| 提示信息 | 无 | 通过 `InputRequest.Prompt` 提供 |
| 使用场景 | 预定义输入序列 | 动态交互式输入 |

## 注意事项

1. **JSON 标记会被过滤**：用户不会看到 `{"pipelinex":"wait-input",...}` 这一行输出
2. **超时处理**：如果配置了 `timeout`，超时后流水线会继续执行（程序可能因等不到输入而失败）
3. **并发安全**：`InputChan` 是线程安全的，可以从多个 goroutine 发送输入
4. **状态检测**：建议定期轮询（如 100ms）检测 PAUSED 状态，避免错过输入请求
