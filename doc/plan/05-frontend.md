# 5. 前端设计

## 5.1 技术栈

| 技术 | 版本 | 用途 |
|------|------|------|
| React | 18.x | UI 框架 |
| TypeScript | 5.x | 类型系统 |
| Vite | 5.x | 构建工具 |
| Tailwind CSS | 3.x | 样式方案 |
| ReactFlow | 11.x | 工作流图可视化 |
| Monaco Editor | 最新 | 代码编辑器 |
| Zustand | 4.x | 状态管理 |
| React Query | 5.x | 服务端状态管理 |
| Axios | 1.x | HTTP 客户端 |

## 5.2 项目结构

```
web/
├── public/
│   └── favicon.ico
├── src/
│   ├── main.tsx              # 应用入口
│   ├── App.tsx               # 根组件
│   ├── index.css             # 全局样式
│   │
│   ├── api/                  # API 客户端
│   │   ├── client.ts         # Axios 实例配置
│   │   ├── nodes.ts          # 节点相关 API
│   │   ├── workflows.ts      # 工作流相关 API
│   │   ├── executions.ts     # 执行相关 API
│   │   ├── ai.ts             # AI 对话 API
│   │   └── config.ts         # 配置相关 API
│   │
│   ├── components/           # 通用组件
│   │   ├── Layout.tsx        # 页面布局
│   │   ├── Sidebar.tsx       # 侧边栏
│   │   ├── Header.tsx        # 顶部导航
│   │   ├── CodeViewer.tsx    # 代码展示（Monaco）
│   │   ├── ParameterList.tsx # 参数列表展示
│   │   ├── LogViewer.tsx     # 日志查看器
│   │   ├── StatusBadge.tsx   # 状态标签
│   │   └── LoadingSpinner.tsx # 加载动画
│   │
│   ├── pages/                # 页面组件
│   │   ├── Dashboard.tsx     # 主控制台
│   │   ├── NodeDetail.tsx    # 节点详情页
│   │   ├── WorkflowDetail.tsx # 工作流详情页
│   │   ├── ExecutionDetail.tsx # 执行详情页
│   │   ├── Settings.tsx      # 设置页
│   │   └── ChatPage.tsx      # AI 对话页
│   │
│   ├── features/             # 功能模块
│   │   ├── chat/             # AI 聊天
│   │   │   ├── ChatPanel.tsx
│   │   │   ├── ChatMessage.tsx
│   │   │   ├── ChatInput.tsx
│   │   │   └── useChat.ts
│   │   │
│   │   ├── workflow-graph/   # 工作流图
│   │   │   ├── WorkflowGraph.tsx
│   │   │   ├── CustomNode.tsx
│   │   │   ├── CustomEdge.tsx
│   │   │   ├── NodeDetailPanel.tsx
│   │   │   └── useWorkflowGraph.ts
│   │   │
│   │   ├── node-editor/      # 节点代码展示
│   │   │   ├── NodeEditor.tsx
│   │   │   ├── CodeTabs.tsx
│   │   │   └── MockRunner.tsx
│   │   │
│   │   └── execution-monitor/ # 执行监控
│   │       ├── ExecutionMonitor.tsx
│   │       ├── ExecutionTimeline.tsx
│   │       ├── ExecutionLogs.tsx
│   │       └── useExecutionStream.ts
│   │
│   ├── hooks/                # 自定义 Hooks
│   │   ├── useSSE.ts         # SSE 实时流
│   │   ├── useNodes.ts       # 节点数据管理
│   │   ├── useWorkflows.ts   # 工作流数据管理
│   │   ├── useExecutions.ts  # 执行数据管理
│   │   └── useAIConfig.ts    # AI 配置管理
│   │
│   ├── stores/               # 状态管理 (Zustand)
│   │   ├── appStore.ts       # 应用级状态
│   │   ├── chatStore.ts      # 聊天状态
│   │   ├── workflowStore.ts  # 工作流状态
│   │   └── executionStore.ts # 执行状态
│   │
│   ├── types/                # TypeScript 类型定义
│   │   ├── node.ts
│   │   ├── workflow.ts
│   │   ├── execution.ts
│   │   ├── ai.ts
│   │   └── api.ts
│   │
│   └── utils/                # 工具函数
│       ├── formatters.ts     # 格式化工具
│       ├── validators.ts     # 验证工具
│       └── constants.ts      # 常量定义
│
├── index.html
├── package.json
├── tsconfig.json
├── vite.config.ts
└── tailwind.config.js
```

## 5.3 页面布局设计

### 5.3.1 主控制台 (Dashboard)

```
┌─────────────────────────────────────────────────────────────────────┐
│  FlowX                                          [设置] [帮助]        │
├──────────────────────────────┬──────────────────────────────────────┤
│                              │                                      │
│  🤖 AI Assistant             │  📊 Workflow Canvas                  │
│                              │                                      │
│  ─────────────────────────   │  ┌────────────────────────────────┐  │
│  User:                       │  │                                │  │
│  帮我做一个图片处理工作流     │  │    ┌──────────┐               │  │
│                              │  │    │ download │──┐            │  │
│  AI:                         │  │    └──────────┘  │            │  │
│  好的！我来帮你创建...        │  │                  ▼            │  │
│  [查看工作流] [Mock测试]      │  │           ┌──────────┐       │  │
│                              │  │           │ compress │       │  │
│  ─────────────────────────   │  │           └──────────┘       │  │
│  User:                       │  │                  │            │  │
│  运行这个工作流               │  │                  ▼            │  │
│                              │  │           ┌──────────┐       │  │
│  AI:                         │  │           │  upload  │       │  │
│  正在执行...                  │  │           └──────────┘       │  │
│  ✓ download - 成功 (2s)      │  │                                │  │
│  ✓ compress - 成功 (3s)      │  └────────────────────────────────┘  │
│  ⏳ upload - 执行中...        │                                      │
│                              │  [运行] [Mock] [查看YAML] [导出]      │
│  [输入需求，按 Enter 发送]    │                                      │
│                              │                                      │
└──────────────────────────────┴──────────────────────────────────────┘
```

**布局说明**：
- **左侧（40%）**：AI 聊天面板，包含对话历史和输入框
- **右侧（60%）**：工作流图可视化 + 操作按钮
- **响应式**：移动端切换为上下布局

### 5.3.2 节点详情页 (Node Detail)

```
┌─────────────────────────────────────────────────────────────────────┐
│  ← 返回节点列表                                               FlowX  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  image_downloader                                    [Mock测试] [删除]│
│  从指定 URL 下载图片                                                  │
│                                                                     │
│  ┌─────────────┐  ┌─────────────────────────────────────────────┐   │
│  │ 参数        │  │ 代码                                        │   │
│  │             │  │                                             │   │
│  │ • url       │  │ import requests                             │   │
│  │   string    │  │ import os                                   │   │
│  │   必需      │  │                                             │   │
│  │             │  │ def download_image(url, output_path):       │   │
│  │ • timeout   │  │     response = requests.get(url, timeout=30)│   │
│  │   integer   │  │     ...                                     │   │
│  │   可选      │  │                                             │   │
│  │   默认: 30  │  │                                             │   │
│  │             │  │                                             │   │
│  │ [Mock测试]  │  │                                             │   │
│  │             │  │                                             │   │
│  └─────────────┘  └─────────────────────────────────────────────┘   │
│                                                                     │
│  标签: #image #download                                             │
│  创建时间: 2025-01-15 10:30                                         │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 5.3.3 执行监控页 (Execution Detail)

```
┌─────────────────────────────────────────────────────────────────────┐
│  ← 返回执行历史                                               FlowX  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  执行 #42 - daily_image_pipeline                                    │
│  状态: ✅ 成功 | 耗时: 2m 15s | 开始: 2025-01-20 10:00              │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ 执行时间线                                                   │    │
│  │                                                              │    │
│  │ [10:00:00] ●──────────────────────────────────● [10:02:15]  │    │
│  │            │ download (30s) │ compress (30s) │ upload (75s)│    │
│  │            │ ✅ 成功        │ ✅ 成功        │ ✅ 成功      │    │
│  │                                                              │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
│  ┌──────────────────────┐  ┌────────────────────────────────────┐  │
│  │ 节点列表             │  │ 日志输出                           │  │
│  │                      │  │                                    │  │
│  │ ○ download          │  │ [10:00:01] 开始下载图片...          │  │
│  │   ✅ 30s            │  │ [10:00:15] 下载进度 50%             │  │
│  │                      │  │ [10:00:30] 下载完成: image.jpg      │  │
│  │ ○ compress          │  │ [10:00:31] 开始压缩...              │  │
│  │   ✅ 30s            │  │ [10:01:00] 压缩完成: 800x600        │  │
│  │                      │  │ [10:01:01] 开始上传...              │  │
│  │ ○ upload            │  │ [10:02:15] 上传成功                 │  │
│  │   ✅ 75s            │  │                                    │  │
│  │                      │  │                                    │  │
│  └──────────────────────┘  └────────────────────────────────────┘  │
│                                                                     │
│  执行结果:                                                          │
│  {
│    "s3_url": "https://bucket.s3.amazonaws.com/nature_20250120.jpg"  │
│  }                                                                  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## 5.4 核心组件设计

### 5.4.1 AI 聊天面板 (ChatPanel)

**职责**：提供用户与 AI 的交互界面

**状态管理**：
```typescript
interface ChatMessage {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  timestamp: Date;
  actions?: AIAction[];  // AI 建议的操作按钮
}

interface AIAction {
  type: 'view_workflow' | 'mock_test' | 'run_workflow' | 'ask_parameter';
  label: string;
  payload?: any;
}
```

**交互设计**：
- 用户输入需求后，显示加载动画
- AI 响应支持 Markdown 渲染
- AI 可返回可操作按钮（如"查看工作流"、"运行 Mock 测试"）
- 支持代码块语法高亮
- 支持流式响应，逐字显示

### 5.4.2 工作流图 (WorkflowGraph)

**职责**：使用 ReactFlow 渲染工作流 DAG 图

**节点设计**：
```typescript
interface WorkflowNodeData {
  id: string;
  name: string;
  description: string;
  status: 'idle' | 'running' | 'success' | 'failed' | 'skipped';
  language?: string;
  duration?: number;
  onViewDetail: () => void;
}
```

**节点样式**：
- 不同状态使用不同颜色（空闲=灰色，运行=蓝色，成功=绿色，失败=红色）
- 运行中的节点显示脉冲动画
- 点击节点展开详情面板
- 支持缩放和平移

**边设计**：
- 直线或贝塞尔曲线连接
- 成功路径：绿色实线
- 失败路径：红色虚线
- 条件边：显示条件标签

### 5.4.3 代码查看器 (CodeViewer)

**职责**：展示节点实现代码和 Mock 代码

**功能**：
- 使用 Monaco Editor，支持 Python/Go/Bash 语法高亮
- 只读模式，不可编辑
- 支持代码折叠
- 显示行号
- 支持主题切换（跟随系统）

### 5.4.4 执行监控器 (ExecutionMonitor)

**职责**：实时展示工作流执行状态和日志

**状态流**：
```
pending → running → (success | failed | cancelled)
```

**实时更新机制**：
- 使用 SSE 接收后端推送的事件
- 节点状态变更时更新图节点颜色
- 新日志到达时自动滚动到底部
- 执行完成后显示结果摘要

## 5.5 状态管理设计

### 5.5.1 应用状态 (appStore)

```typescript
interface AppState {
  // 主题
  theme: 'light' | 'dark' | 'system';
  
  // 侧边栏
  sidebarOpen: boolean;
  
  // 当前页面
  currentPage: 'dashboard' | 'nodes' | 'workflows' | 'executions' | 'settings';
  
  // 全局加载状态
  globalLoading: boolean;
  
  // 系统配置
  systemConfig: SystemConfig;
}
```

### 5.5.2 聊天状态 (chatStore)

```typescript
interface ChatState {
  sessions: ChatSession[];
  activeSessionId: string | null;
  isGenerating: boolean;
  streamingContent: string;
}

interface ChatSession {
  id: string;
  title: string;
  messages: ChatMessage[];
  createdAt: Date;
}
```

### 5.5.3 工作流状态 (workflowStore)

```typescript
interface WorkflowState {
  workflows: Workflow[];
  currentWorkflow: Workflow | null;
  isGenerating: boolean;
  generatedYAML: string | null;
}
```

### 5.5.4 执行状态 (executionStore)

```typescript
interface ExecutionState {
  executions: Execution[];
  currentExecution: Execution | null;
  isStreaming: boolean;
  logs: ExecutionLog[];
  nodeStatuses: Record<string, NodeStatus>;
}
```

## 5.6 实时通信设计

### 5.6.1 SSE 连接管理

```typescript
// hooks/useSSE.ts
function useExecutionStream(executionId: string) {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [status, setStatus] = useState<ExecutionStatus>('pending');
  
  useEffect(() => {
    const eventSource = new EventSource(`/api/v1/executions/${executionId}/stream`);
    
    eventSource.onmessage = (event) => {
      const data = JSON.parse(event.data);
      handleEvent(data);
    };
    
    return () => eventSource.close();
  }, [executionId]);
  
  return { logs, status };
}
```

### 5.6.2 事件处理

前端需要处理以下 SSE 事件：

| 事件类型 | 处理逻辑 |
|---------|---------|
| `execution_start` | 初始化执行状态，清空日志 |
| `node_start` | 更新对应节点状态为 "running" |
| `node_log` | 追加日志到日志列表 |
| `node_complete` | 更新节点状态为 "success" 或 "failed" |
| `execution_complete` | 更新整体状态，显示结果 |
| `execution_error` | 显示错误信息，标记失败节点 |

## 5.7 路由设计

使用 React Router 进行前端路由管理：

```typescript
const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    children: [
      { path: '/', element: <Dashboard /> },           // 主控制台
      { path: '/nodes', element: <NodeList /> },       // 节点列表
      { path: '/nodes/:id', element: <NodeDetail /> }, // 节点详情
      { path: '/workflows', element: <WorkflowList /> },
      { path: '/workflows/:id', element: <WorkflowDetail /> },
      { path: '/executions', element: <ExecutionList /> },
      { path: '/executions/:id', element: <ExecutionDetail /> },
      { path: '/settings', element: <Settings /> },    // 设置页
    ]
  }
]);
```

## 5.8 错误处理设计

### 5.8.1 API 错误处理

使用 Axios 拦截器统一处理错误：

```typescript
// 全局错误处理
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 502) {
      showToast('AI 服务暂时不可用，请检查配置');
    } else if (error.response?.status === 422) {
      showToast('请求无法处理：' + error.response.data.error.detail);
    } else {
      showToast('请求失败：' + error.message);
    }
    return Promise.reject(error);
  }
);
```

### 5.8.2 边界错误处理

使用 React Error Boundary 捕获组件错误：

```typescript
class ErrorBoundary extends React.Component {
  state = { hasError: false, error: null };
  
  static getDerivedStateFromError(error) {
    return { hasError: true, error };
  }
  
  render() {
    if (this.state.hasError) {
      return <ErrorFallback error={this.state.error} />;
    }
    return this.props.children;
  }
}
```

## 5.9 性能优化策略

1. **虚拟滚动**：节点列表和执行日志使用虚拟滚动，避免大量 DOM 节点
2. **懒加载**：工作流图节点懒加载，首次只渲染可见区域
3. **防抖**：搜索输入使用防抖，减少 API 请求
4. **缓存**：React Query 自动缓存和失效管理
5. **代码分割**：路由级代码分割，按需加载页面组件
6. **构建优化**：Vite 开启 tree-shaking 和压缩

## 5.10 主题与样式

### 5.10.1 设计系统

基于 Tailwind CSS 定义设计令牌：

```javascript
// tailwind.config.js
module.exports = {
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#eff6ff',
          500: '#3b82f6',
          600: '#2563eb',
          700: '#1d4ed8',
        },
        status: {
          success: '#22c55e',
          running: '#3b82f6',
          failed: '#ef4444',
          pending: '#6b7280',
        }
      }
    }
  }
};
```

### 5.10.2 暗色模式

使用 Tailwind 的 `dark:` 前缀实现暗色模式：

```html
<div class="bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100">
  <!-- 内容 -->
</div>
```

通过 `class` 策略切换，状态保存在 localStorage。
