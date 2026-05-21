# 6. AI 服务层设计

## 6.1 设计目标

- **多提供商支持**：统一接口封装 OpenAI、Anthropic、Ollama 等
- **模型能力抽象**：自动识别模型能力（函数调用、JSON 模式等）
- **Prompt 工程**：标准化 Prompt 模板，确保 AI 输出符合 FlowX 规范
- **容错机制**：重试、降级、超时控制
- **流式响应**：支持 SSE 流式输出，提升用户体验

## 6.2 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      AI Service Layer                        │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                    AIService                           │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐  │  │
│  │  │ NodeGenerator│  │WorkflowGen   │  │  ChatService│  │  │
│  │  └──────────────┘  └──────────────┘  └─────────────┘  │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                    Provider Interface                  │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ │  │
│  │  │  OpenAI  │ │Anthropic │ │  Ollama  │ │  Custom  │ │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                    Prompt Engine                       │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐  │  │
│  │  │   Templates  │  │  Variables   │  │   Parser    │  │  │
│  │  └──────────────┘  └──────────────┘  └─────────────┘  │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## 6.3 Provider 接口设计

```go
// Provider 是所有 AI 提供商的通用接口
type Provider interface {
    // 基本信息
    Name() string
    AvailableModels() []ModelInfo
    
    // 核心能力
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    ChatStream(ctx context.Context, req ChatRequest) (StreamIterator, error)
    
    // 健康检查
    HealthCheck(ctx context.Context) error
}

type ModelInfo struct {
    Name                  string
    DisplayName           string
    MaxContextLength      int
    SupportsFunctionCall  bool
    SupportsJSONMode      bool
    SupportsVision        bool
    MaxOutputTokens       int
}

type ChatRequest struct {
    Model       string
    Messages    []Message
    Temperature float64
    MaxTokens   int
    JSONMode    bool  // 要求返回 JSON 格式
}

type Message struct {
    Role    string // system | user | assistant
    Content string
}

type ChatResponse struct {
    Content      string
    Usage        TokenUsage
    FinishReason string
}

type StreamIterator interface {
    Next() (StreamChunk, error)
    Close() error
}

type StreamChunk struct {
    Content string
    Done    bool
}
```

## 6.4 提供商实现

### 6.4.1 OpenAI Provider

```go
type OpenAIProvider struct {
    apiKey   string
    baseURL  string // 支持自定义代理地址
    client   *http.Client
}

func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
    // 构建 OpenAI 格式的请求
    // 调用 /v1/chat/completions
    // 解析响应为通用 ChatResponse
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, req ChatRequest) (StreamIterator, error) {
    // 调用 /v1/chat/completions?stream=true
    // 返回 SSE 流迭代器
}
```

### 6.4.2 Anthropic Provider

```go
type AnthropicProvider struct {
    apiKey  string
    baseURL string
    client  *http.Client
}

// Anthropic API 格式与 OpenAI 不同，需要转换：
// - messages 格式差异
// - system prompt 位置不同
// - 响应结构差异
// 在 Provider 内部完成所有转换，对外暴露统一接口
```

### 6.4.3 Ollama Provider

```go
type OllamaProvider struct {
    baseURL string // 默认 http://localhost:11434
    client  *http.Client
}

// Ollama 特点：
// - 本地运行，无需 API Key
// - 模型需要预先拉取
// - API 兼容 OpenAI 格式（/api/chat）
// - 支持流式响应
```

## 6.5 Prompt 工程

### 6.5.1 节点生成 Prompt

```markdown
你是一个 FlowX 节点开发专家。FlowX 是一个工作流引擎，节点是可复用的执行单元。

## 任务
根据用户描述，生成一个符合 FlowX 规范的节点。

## 节点规范
1. **输入**：节点通过环境变量接收参数，参数名使用大写+下划线格式，如 `URL`、`TIMEOUT`
2. **输出**：节点将结果输出到 stdout，使用 JSON 格式
3. **错误**：错误信息输出到 stderr，进程 exit code 非 0
4. **语言**：支持 Python、Go、Bash 等

## 输出格式
必须以 JSON 格式返回，包含以下字段：
- name: 节点名称（蛇形命名，如 image_downloader）
- description: 节点描述
- language: 编程语言
- parameters: 参数列表，每个参数包含 name, type, description, required, default
- code: 完整实现代码
- mock_code: Mock 测试代码（不依赖外部服务，返回模拟数据）

## 用户描述
{{user_description}}

## 上下文
{{context}}

请生成节点：
```

### 6.5.2 工作流生成 Prompt

```markdown
你是一个工作流编排专家。FlowX 使用 YAML 配置定义工作流，图结构使用 Mermaid stateDiagram-v2 语法。

## 可用节点
{{available_nodes}}

## 用户需求
{{user_description}}

## 输出要求
1. 生成完整的 FlowX YAML 配置
2. 如果现有节点不够用，列出需要新建的节点
3. 图结构必须正确，使用 Mermaid stateDiagram-v2 语法
4. 节点配置必须引用正确的参数

## YAML 结构示例
```yaml
name: workflow_name
version: "1.0"
graph: |
  stateDiagram-v2
    [*] --> node1
    node1 --> node2
    node2 --> [*]
nodes:
  node1:
    steps:
      - name: step1
        executor: local
        commands:
          - python {{.node.code}}
```

请生成工作流配置：
```

### 6.5.3 对话 Prompt

```markdown
你是 FlowX AI 助手，一个帮助用户创建和管理工作流的智能助手。

## 能力
1. 理解用户的自动化需求
2. 生成可复用的工作流节点
3. 编排复杂的工作流
4. 诊断和修复工作流错误

## 原则
1. 主动询问缺失的关键信息
2. 提供清晰的操作建议
3. 如果用户描述模糊，给出示例帮助澄清
4. 保持对话简洁，聚焦工作流主题

## 当前上下文
{{context}}

用户消息：{{user_message}}
```

## 6.6 高阶服务封装

### 6.6.1 NodeGenerator

```go
type NodeGenerator struct {
    aiService *AIService
    nodeRepo  db.NodeRepository
}

// GenerateNode 根据用户描述生成节点
func (g *NodeGenerator) GenerateNode(ctx context.Context, description string, opts GenerateOptions) (*db.Node, error) {
    // 1. 组装 Prompt（包含节点规范和上下文）
    prompt := g.buildPrompt(description, opts)
    
    // 2. 调用 AI，要求 JSON 模式输出
    response, err := g.aiService.Chat(ctx, ChatRequest{
        Messages: []Message{{Role: "user", Content: prompt}},
        JSONMode: true,
    })
    
    // 3. 解析 JSON 响应
    var nodeData GeneratedNode
    if err := json.Unmarshal([]byte(response.Content), &nodeData); err != nil {
        return nil, fmt.Errorf("parse node data failed: %w", err)
    }
    
    // 4. 验证节点数据完整性
    if err := g.validateNode(nodeData); err != nil {
        return nil, fmt.Errorf("validate node failed: %w", err)
    }
    
    // 5. 保存到数据库
    node := &db.Node{
        Name:        nodeData.Name,
        Description: nodeData.Description,
        Language:    nodeData.Language,
        Code:        nodeData.Code,
        MockCode:    nodeData.MockCode,
        Parameters:  nodeData.Parameters,
    }
    
    if err := g.nodeRepo.Create(ctx, node); err != nil {
        return nil, fmt.Errorf("save node failed: %w", err)
    }
    
    return node, nil
}
```

### 6.6.2 WorkflowGenerator

```go
type WorkflowGenerator struct {
    aiService     *AIService
    nodeRepo      db.NodeRepository
    workflowRepo  db.WorkflowRepository
}

// GenerateWorkflow 根据用户描述生成工作流
func (g *WorkflowGenerator) GenerateWorkflow(ctx context.Context, description string, opts GenerateOptions) (*db.Workflow, error) {
    // 1. 获取所有可用节点作为上下文
    nodes, _ := g.nodeRepo.List(ctx)
    
    // 2. 组装 Prompt
    prompt := g.buildPrompt(description, nodes, opts)
    
    // 3. 调用 AI 生成
    response, err := g.aiService.Chat(ctx, ChatRequest{
        Messages: []Message{{Role: "user", Content: prompt}},
        JSONMode: false, // YAML 不是 JSON
    })
    
    // 4. 从响应中提取 YAML
    yamlConfig := g.extractYAML(response.Content)
    
    // 5. 验证 YAML 结构
    if err := g.validateYAML(yamlConfig); err != nil {
        return nil, fmt.Errorf("validate yaml failed: %w", err)
    }
    
    // 6. 保存工作流
    workflow := &db.Workflow{
        Name:       g.generateName(description),
        Description: description,
        YAMLConfig: yamlConfig,
        Status:     "draft",
    }
    
    if err := g.workflowRepo.Create(ctx, workflow); err != nil {
        return nil, fmt.Errorf("save workflow failed: %w", err)
    }
    
    return workflow, nil
}
```

## 6.7 容错与优化

### 6.7.1 重试机制

```go
type RetryConfig struct {
    MaxRetries  int
    BaseDelay   time.Duration
    MaxDelay    time.Duration
    Multiplier  float64
}

func (p *BaseProvider) callWithRetry(ctx context.Context, fn func() error) error {
    var lastErr error
    delay := p.retryConfig.BaseDelay
    
    for i := 0; i <= p.retryConfig.MaxRetries; i++ {
        if err := fn(); err != nil {
            lastErr = err
            
            // 判断是否需要重试
            if !isRetryable(err) {
                return err
            }
            
            // 指数退避
            if i < p.retryConfig.MaxRetries {
                time.Sleep(delay)
                delay = min(time.Duration(float64(delay)*p.retryConfig.Multiplier), p.retryConfig.MaxDelay)
            }
        } else {
            return nil
        }
    }
    
    return fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

### 6.7.2 故障转移

```go
type AIService struct {
    providers []Provider
    // 按优先级排序，主模型在前
}

func (s *AIService) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
    for _, provider := range s.providers {
        if !provider.IsEnabled() {
            continue
        }
        
        resp, err := provider.Chat(ctx, req)
        if err == nil {
            return resp, nil
        }
        
        // 记录失败，尝试下一个提供商
        log.Printf("Provider %s failed: %v", provider.Name(), err)
    }
    
    return ChatResponse{}, fmt.Errorf("all providers failed")
}
```

### 6.7.3 流式响应处理

```go
func (s *AIService) ChatStream(ctx context.Context, req ChatRequest, onChunk func(string)) error {
    iterator, err := s.primaryProvider.ChatStream(ctx, req)
    if err != nil {
        return err
    }
    defer iterator.Close()
    
    for {
        chunk, err := iterator.Next()
        if err != nil {
            return err
        }
        
        if chunk.Done {
            break
        }
        
        onChunk(chunk.Content)
    }
    
    return nil
}
```

## 6.8 AI 配置管理

### 6.8.1 配置模型

```go
type AIConfig struct {
    ID           int64
    Provider     string  // openai | anthropic | ollama
    Name         string  // 用户自定义名称
    Model        string  // 模型名称
    APIKey       string  // 加密存储
    BaseURL      string  // 自定义地址
    Temperature  float64
    MaxTokens    int
    IsActive     bool    // 是否为默认配置
    IsEnabled    bool    // 是否启用
    Capabilities string  // JSON 模型能力
}
```

### 6.8.2 配置优先级

1. 用户显式指定的配置
2. `is_active = true` 的默认配置
3. 第一个启用的配置
4. 本地 Ollama（如果可用）

### 6.8.3 API Key 加密

使用 AES-GCM 加密存储 API Key：

```go
// 加密密钥从环境变量或系统密钥链获取
var encryptionKey = os.Getenv("FLOWX_ENCRYPTION_KEY")

func encryptAPIKey(key string) (string, error) {
    block, err := aes.NewCipher([]byte(encryptionKey))
    if err != nil {
        return "", err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }
    
    ciphertext := gcm.Seal(nonce, nonce, []byte(key), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}
```

## 6.9 监控与日志

### 6.9.1 AI 调用日志

记录每次 AI 调用的关键信息：

```sql
CREATE TABLE ai_call_logs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    provider        TEXT NOT NULL,
    model           TEXT NOT NULL,
    operation       TEXT NOT NULL,        -- generate_node | generate_workflow | chat
    input_tokens    INTEGER,
    output_tokens   INTEGER,
    latency_ms      INTEGER,
    success         BOOLEAN,
    error_message   TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 6.9.2 性能指标

监控以下指标：
- AI 调用延迟（P50、P95、P99）
- Token 使用量
- 生成成功率
- 各提供商可用性

## 6.10 安全考虑

1. **API Key 保护**：加密存储，不在日志中明文输出
2. **输入过滤**：对用户输入进行基本的 XSS 和注入过滤
3. **输出验证**：AI 生成的代码在存储前进行语法验证
4. **沙箱执行**：Mock 测试在隔离环境运行，防止恶意代码
5. **速率限制**：对 AI 调用进行速率限制，防止滥用
