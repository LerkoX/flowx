# 5. 前端设计（详细实现方案）

## 5.1 技术栈

| 技术 | 版本 | 用途 |
|------|------|------|
| React | 18.x | UI 框架 |
| TypeScript | 5.x | 类型系统 |
| Vite | 5.x | 构建工具 |
| Tailwind CSS | 3.x | 样式方案 |
| ReactFlow (`@xyflow/react`) | 12.x | 工作流图可视化 |
| Monaco Editor | 最新 | 代码编辑器 |
| Zustand | 4.x | 状态管理 |
| React Query | 5.x | 服务端状态管理 |
| Axios | 1.x | HTTP 客户端 |
| `react-markdown` | 9.x | Markdown 渲染 |
| `react-syntax-highlighter` | 15.x | 代码语法高亮 |
| `dagre` | 0.8.x | DAG 自动布局算法 |
| `idb-keyval` | 6.x | IndexedDB 封装（本地缓存） |

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
│   │   ├── chat/             # AI 聊天模块
│   │   │   ├── ChatPanel.tsx           # 聊天面板主组件
│   │   │   ├── ChatMessage.tsx         # 单条消息组件
│   │   │   ├── ChatInput.tsx           # 输入框组件
│   │   │   ├── StreamingText.tsx       # 流式文本渲染
│   │   │   ├── AIActionButtons.tsx     # AI 操作按钮
│   │   │   ├── useChat.ts              # 聊天逻辑 Hook
│   │   │   ├── useChatContext.ts       # 上下文管理 Hook
│   │   │   ├── useChatCache.ts         # 缓存管理 Hook
│   │   │   └── contextCompressor.ts    # 上下文压缩器
│   │   │
│   │   ├── workflow-graph/   # 工作流图模块
│   │   │   ├── WorkflowGraph.tsx       # 图主组件
│   │   │   ├── CustomNode.tsx          # 自定义节点
│   │   │   ├── CustomEdge.tsx          # 自定义边
│   │   │   ├── NodeDetailPanel.tsx     # 节点详情面板
│   │   │   ├── AutoLayout.ts           # 自动布局算法
│   │   │   ├── GraphStatusSync.ts      # 状态同步逻辑
│   │   │   └── useWorkflowGraph.ts     # 图数据 Hook
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
│   ├── utils/                # 工具函数
│   │   ├── formatters.ts     # 格式化工具
│   │   ├── validators.ts     # 验证工具
│   │   ├── tokenEstimator.ts # Token 估算器
│   │   └── constants.ts      # 常量定义
│   │
│   └── services/             # 业务服务层
│       ├── chatContext.ts    # 上下文管理服务
│       ├── chatCache.ts      # 缓存服务
│       └── layoutEngine.ts   # 布局引擎
│
├── index.html
├── package.json
├── tsconfig.json
├── vite.config.ts
└── tailwind.config.js
```

---

## 5.3 工作流图显示组件（ReactFlow 详细方案）

### 5.3.1 方案选型：ReactFlow

**选择 ReactFlow 而非其他方案的理由**：
- **行业标准**：n8n、ComfyUI、LangFlow、Dify 等主流工作流平台均采用 ReactFlow
- **功能完备**：支持自定义节点/边、动画、交互、缩放、拖拽画布
- **readOnly 模式**：完美契合"只展示不编辑"的需求
- **状态驱动**：节点数据完全由 React state 控制，方便对接 SSE 实时更新
- **性能优秀**：仅渲染可视区域节点，支持大量节点流畅展示

### 5.3.2 自定义节点组件

```typescript
// features/workflow-graph/CustomNode.tsx
import { memo } from 'react';
import { Handle, Position, NodeProps } from '@xyflow/react';

interface FlowxNodeData {
  id: string;
  name: string;
  description: string;
  status: 'idle' | 'running' | 'success' | 'failed' | 'skipped';
  language?: string;
  duration?: number; // 毫秒
  onViewDetail: () => void;
}

const statusColors = {
  idle: 'border-gray-300 bg-gray-50',
  running: 'border-blue-500 bg-blue-50 animate-pulse',
  success: 'border-green-500 bg-green-50',
  failed: 'border-red-500 bg-red-50',
  skipped: 'border-gray-200 bg-gray-100 opacity-50',
};

const statusIcons = {
  idle: '○',
  running: '⟳',
  success: '✓',
  failed: '✗',
  skipped: '⊘',
};

const FlowxNode = memo(({ data, selected }: NodeProps<FlowxNodeData>) => {
  const { name, description, status, language, duration, onViewDetail } = data;

  return (
    <div
      className={`
        rounded-lg border-2 px-4 py-3 min-w-[160px] max-w-[280px]
        shadow-sm transition-all duration-300 cursor-pointer
        ${statusColors[status]}
        ${selected ? 'ring-2 ring-offset-2 ring-blue-400' : ''}
      `}
      onClick={onViewDetail}
    >
      {/* 输入连接点 */}
      <Handle
        type="target"
        position={Position.Top}
        className="w-3 h-3 bg-gray-400"
      />

      {/* 节点内容 */}
      <div className="flex items-start gap-2">
        <span className="text-lg font-bold mt-0.5">{statusIcons[status]}</span>
        <div className="flex-1 min-w-0">
          <div className="font-semibold text-sm truncate">{name}</div>
          {description && (
            <div className="text-xs text-gray-500 truncate mt-0.5">
              {description}
            </div>
          )}
          <div className="flex items-center gap-2 mt-1.5">
            {language && (
              <span className="text-[10px] px-1.5 py-0.5 bg-gray-200 rounded">
                {language}
              </span>
            )}
            {duration !== undefined && (
              <span className="text-[10px] text-gray-500">
                {formatDuration(duration)}
              </span>
            )}
          </div>
        </div>
      </div>

      {/* 输出连接点 */}
      <Handle
        type="source"
        position={Position.Bottom}
        className="w-3 h-3 bg-gray-400"
      />
    </div>
  );
});

FlowxNode.displayName = 'FlowxNode';

export default FlowxNode;
```

### 5.3.3 自定义边组件

```typescript
// features/workflow-graph/CustomEdge.tsx
import { memo } from 'react';
import { EdgeProps, getBezierPath } from '@xyflow/react';

const FlowxEdge = memo(({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
  markerEnd,
}: EdgeProps) => {
  const [edgePath] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  const isAnimated = data?.animated || false;
  const isFailed = data?.status === 'failed';

  return (
    <>
      <path
        id={id}
        className={`
          react-flow__edge-path
          ${isFailed ? 'stroke-red-400 stroke-dasharray-4' : 'stroke-gray-400'}
          ${isAnimated ? 'animate-dash' : ''}
        `}
        d={edgePath}
        strokeWidth={2}
        fill="none"
        markerEnd={markerEnd}
      />
      {data?.label && (
        <text
          className="text-xs fill-gray-500"
          style={{
            fontSize: '10px',
          }}
        >
          <textPath
            href={`#${id}`}
            startOffset="50%"
            textAnchor="middle"
          >
            {data.label}
          </textPath>
        </text>
      )}
    </>
  );
});

FlowxEdge.displayName = 'FlowxEdge';

export default FlowxEdge;
```

### 5.3.4 自动布局算法（dagre）

```typescript
// features/workflow-graph/AutoLayout.ts
import dagre from 'dagre';
import { Node, Edge } from '@xyflow/react';

const NODE_WIDTH = 180;
const NODE_HEIGHT = 80;
const RANK_SEP = 80;
const NODE_SEP = 50;

interface LayoutOptions {
  direction?: 'TB' | 'LR'; // Top-Bottom 或 Left-Right
  align?: 'UL' | 'UR' | 'DL' | 'DR';
}

/**
 * 使用 dagre 自动计算 DAG 布局
 */
export function autoLayout(
  nodes: Node[],
  edges: Edge[],
  options: LayoutOptions = {}
): { nodes: Node[]; edges: Edge[] } {
  const { direction = 'TB', align = 'UL' } = options;

  const dagreGraph = new dagre.graphlib.Graph();
  dagreGraph.setDefaultEdgeLabel(() => ({}));
  dagreGraph.setGraph({
    rankdir: direction,
    align,
    nodesep: NODE_SEP,
    ranksep: RANK_SEP,
  });

  // 添加节点到 dagre
  nodes.forEach((node) => {
    dagreGraph.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  });

  // 添加边到 dagre
  edges.forEach((edge) => {
    dagreGraph.setEdge(edge.source, edge.target);
  });

  // 执行布局计算
  dagre.layout(dagreGraph);

  // 将 dagre 计算的位置应用到 ReactFlow 节点
  const layoutedNodes = nodes.map((node) => {
    const nodeWithPosition = dagreGraph.node(node.id);
    return {
      ...node,
      position: {
        x: nodeWithPosition.x - NODE_WIDTH / 2,
        y: nodeWithPosition.y - NODE_HEIGHT / 2,
      },
    };
  });

  return { nodes: layoutedNodes, edges };
}

/**
 * 将 FlowX YAML 中的 Mermaid 图转换为 ReactFlow 节点和边
 */
export function parseMermaidToFlow(
  graph: string,
  nodeStatusMap: Record<string, string> = {}
): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = [];
  const edges: Edge[] = [];
  const nodeSet = new Set<string>();

  // 解析 Mermaid stateDiagram-v2 语法
  const lines = graph.split('\n');

  lines.forEach((line) => {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('stateDiagram')) return;

    // 匹配 [*] --> nodeId 或 nodeId --> [*]
    const startEndMatch = trimmed.match(/\[\*\]\s*-->\s*(\w+)|(\w+)\s*-->\s*\[\*\]/);
    if (startEndMatch) {
      const nodeId = startEndMatch[1] || startEndMatch[2];
      if (!nodeSet.has(nodeId)) {
        nodes.push(createNode(nodeId, nodeStatusMap[nodeId]));
        nodeSet.add(nodeId);
      }
      return;
    }

    // 匹配 nodeA --> nodeB
    const edgeMatch = trimmed.match(/(\w+)\s*-->\s*(\w+)(?:\s*:\s*(.+))?/);
    if (edgeMatch) {
      const [, source, target, label] = edgeMatch;

      if (!nodeSet.has(source)) {
        nodes.push(createNode(source, nodeStatusMap[source]));
        nodeSet.add(source);
      }
      if (!nodeSet.has(target)) {
        nodes.push(createNode(target, nodeStatusMap[target]));
        nodeSet.add(target);
      }

      edges.push({
        id: `e-${source}-${target}`,
        source,
        target,
        label: label?.trim(),
        type: 'flowxEdge',
      });
    }
  });

  return { nodes, edges };
}

function createNode(id: string, status?: string): Node {
  return {
    id,
    type: 'flowxNode',
    position: { x: 0, y: 0 }, // 位置由 autoLayout 计算
    data: {
      id,
      name: id,
      description: '',
      status: (status as any) || 'idle',
    },
  };
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${(ms / 60000).toFixed(1)}m`;
}
```

### 5.3.5 图主组件

```typescript
// features/workflow-graph/WorkflowGraph.tsx
import { useCallback, useEffect, useState } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  Panel,
  ReactFlowProvider,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';

import FlowxNode from './CustomNode';
import FlowxEdge from './CustomEdge';
import { autoLayout, parseMermaidToFlow } from './AutoLayout';
import { useWorkflowGraph } from './useWorkflowGraph';

const nodeTypes = { flowxNode: FlowxNode };
const edgeTypes = { flowxEdge: FlowxEdge };

interface WorkflowGraphProps {
  workflowId: string;
  yamlConfig: string;
  readOnly?: boolean;
}

function WorkflowGraphInner({ workflowId, yamlConfig, readOnly = true }: WorkflowGraphProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [selectedNode, setSelectedNode] = useState<string | null>(null);

  const { executionStatus, nodeStatuses } = useWorkflowGraph(workflowId);

  // 初始加载：解析 YAML 并布局
  useEffect(() => {
    if (!yamlConfig) return;

    try {
      // 1. 解析 YAML 中的 graph 字段
      const config = parseYaml(yamlConfig);
      const { nodes: rawNodes, edges: rawEdges } = parseMermaidToFlow(
        config.graph,
        nodeStatuses
      );

      // 2. 自动布局
      const { nodes: layoutedNodes, edges: layoutedEdges } = autoLayout(
        rawNodes,
        rawEdges,
        { direction: 'TB' }
      );

      setNodes(layoutedNodes);
      setEdges(layoutedEdges);
    } catch (err) {
      console.error('Failed to parse workflow graph:', err);
    }
  }, [yamlConfig, setNodes, setEdges]);

  // 实时同步执行状态
  useEffect(() => {
    if (!nodeStatuses || Object.keys(nodeStatuses).length === 0) return;

    setNodes((nds) =>
      nds.map((node) => {
        const status = nodeStatuses[node.id];
        if (status && status !== node.data.status) {
          return {
            ...node,
            data: {
              ...node.data,
              status,
              duration: status === 'success' || status === 'failed'
                ? calculateDuration(node.data.startTime)
                : node.data.duration,
            },
          };
        }
        return node;
      })
    );

    // 动画效果：运行中的边
    setEdges((eds) =>
      eds.map((edge) => {
        const sourceStatus = nodeStatuses[edge.source];
        return {
          ...edge,
          data: {
            ...edge.data,
            animated: sourceStatus === 'running',
            status: nodeStatuses[edge.target] === 'failed' ? 'failed' : 'normal',
          },
        };
      })
    );
  }, [nodeStatuses, setNodes, setEdges]);

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setSelectedNode(node.id);
  }, []);

  const onPaneClick = useCallback(() => {
    setSelectedNode(null);
  }, []);

  return (
    <div className="w-full h-full relative">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={readOnly ? undefined : onNodesChange}
        onEdgesChange={readOnly ? undefined : onEdgesChange}
        onNodeClick={onNodeClick}
        onPaneClick={onPaneClick}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        fitView
        fitViewOptions={{ padding: 0.2 }}
        minZoom={0.2}
        maxZoom={2}
        nodesDraggable={!readOnly}
        nodesConnectable={!readOnly}
        elementsSelectable={true}
      >
        <Background color="#e5e7eb" gap={20} size={1} />
        <Controls />
        <MiniMap
          nodeColor={(node) => {
            const statusColors = {
              idle: '#d1d5db',
              running: '#60a5fa',
              success: '#4ade80',
              failed: '#f87171',
              skipped: '#9ca3af',
            };
            return statusColors[node.data?.status as string] || '#d1d5db';
          }}
        />
        <Panel position="top-right" className="bg-white rounded-lg shadow p-2">
          <div className="text-xs text-gray-500">
            状态: {executionStatus || 'idle'}
          </div>
        </Panel>
      </ReactFlow>

      {/* 节点详情浮层 */}
      {selectedNode && (
        <NodeDetailPanel
          nodeId={selectedNode}
          onClose={() => setSelectedNode(null)}
        />
      )}
    </div>
  );
}

// 包装 Provider
export default function WorkflowGraph(props: WorkflowGraphProps) {
  return (
    <ReactFlowProvider>
      <WorkflowGraphInner {...props} />
    </ReactFlowProvider>
  );
}
```

### 5.3.6 状态同步 Hook

```typescript
// features/workflow-graph/useWorkflowGraph.ts
import { useEffect } from 'react';
import { useExecutionStore } from '@/stores/executionStore';

interface UseWorkflowGraphResult {
  executionStatus: string | null;
  nodeStatuses: Record<string, string>;
}

export function useWorkflowGraph(workflowId: string): UseWorkflowGraphResult {
  const { currentExecution, isStreaming, nodeStatuses } = useExecutionStore();

  // SSE 连接已在 useExecutionStream 中管理
  // 这里只需从 store 读取状态

  return {
    executionStatus: currentExecution?.status || null,
    nodeStatuses: nodeStatuses || {},
  };
}
```

---

## 5.4 AI 聊天组件（自建详细方案）

### 5.4.1 方案选型：自建而非第三方库

**选择自建的理由**：
- **AI 操作按钮**：需要在消息中嵌入自定义操作按钮（如"查看工作流"、"Mock 测试"），第三方库难以扩展
- **流式渲染**：需要精确的打字机效果控制，第三方库往往过于封装
- **上下文管理**：需要深度集成自定义的上下文压缩和缓存逻辑
- **设计一致性**：使用 shadcn/ui 组件拼装，与整体设计系统保持一致
- **状态耦合**：聊天状态与工作流、执行状态高度耦合，自建更灵活

### 5.4.2 聊天面板主组件

```typescript
// features/chat/ChatPanel.tsx
import { useRef, useEffect } from 'react';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useChatStore } from '@/stores/chatStore';
import ChatMessage from './ChatMessage';
import ChatInput from './ChatInput';
import StreamingText from './StreamingText';

export default function ChatPanel() {
  const scrollRef = useRef<HTMLDivElement>(null);
  const {
    activeSession,
    messages,
    isGenerating,
    streamingContent,
    sendMessage,
    executeAction,
  } = useChatStore();

  // 自动滚动到底部
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, streamingContent]);

  return (
    <div className="flex flex-col h-full bg-gray-50 dark:bg-gray-900">
      {/* 消息列表 */}
      <ScrollArea ref={scrollRef} className="flex-1 px-4 py-4">
        <div className="space-y-4">
          {/* 欢迎消息 */}
          {messages.length === 0 && (
            <WelcomeMessage />
          )}

          {/* 历史消息 */}
          {messages.map((message) => (
            <ChatMessage
              key={message.id}
              message={message}
              onActionClick={executeAction}
            />
          ))}

          {/* 流式输出 */}
          {isGenerating && streamingContent && (
            <StreamingText content={streamingContent} />
          )}

          {/* 加载指示器 */}
          {isGenerating && !streamingContent && (
            <LoadingIndicator />
          )}
        </div>
      </ScrollArea>

      {/* 输入框 */}
      <div className="border-t border-gray-200 dark:border-gray-700 p-4 bg-white dark:bg-gray-800">
        <ChatInput
          onSend={sendMessage}
          disabled={isGenerating}
          placeholder="描述你想要的工作流，例如：帮我做一个图片处理流水线..."
        />
      </div>
    </div>
  );
}

function WelcomeMessage() {
  return (
    <div className="text-center py-8">
      <div className="text-4xl mb-4">🤖</div>
      <h2 className="text-lg font-semibold mb-2">FlowX AI 助手</h2>
      <p className="text-sm text-gray-500 max-w-md mx-auto">
        告诉我你想要什么工作流，我会帮你生成节点、编排流程，还能 Mock 测试。
        <br />
        试试说："帮我做一个每天自动下载图片并压缩的工作流"
      </p>
    </div>
  );
}

function LoadingIndicator() {
  return (
    <div className="flex items-center gap-2 text-gray-500 text-sm">
      <div className="flex gap-1">
        <span className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
        <span className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
        <span className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
      </div>
      <span>AI 正在思考...</span>
    </div>
  );
}
```

### 5.4.3 单条消息组件

```typescript
// features/chat/ChatMessage.tsx
import { memo } from 'react';
import ReactMarkdown from 'react-markdown';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { User, Bot } from 'lucide-react';
import { ChatMessage as ChatMessageType, AIAction } from '@/types/ai';
import AIActionButtons from './AIActionButtons';

interface ChatMessageProps {
  message: ChatMessageType;
  onActionClick: (action: AIAction) => void;
}

const ChatMessage = memo(({ message, onActionClick }: ChatMessageProps) => {
  const { role, content, timestamp, actions } = message;
  const isUser = role === 'user';

  return (
    <div className={`flex gap-3 ${isUser ? 'flex-row-reverse' : 'flex-row'}`}>
      {/* 头像 */}
      <div className={`
        w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0
        ${isUser ? 'bg-blue-500 text-white' : 'bg-gray-200 dark:bg-gray-700'}
      `}>
        {isUser ? <User size={16} /> : <Bot size={16} />}
      </div>

      {/* 内容 */}
      <div className={`flex flex-col ${isUser ? 'items-end' : 'items-start'} max-w-[80%]`}>
        {/* 消息气泡 */}
        <div className={`
          rounded-2xl px-4 py-2.5
          ${isUser
            ? 'bg-blue-500 text-white rounded-tr-sm'
            : 'bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-tl-sm shadow-sm'
          }
        `}>
          <MarkdownContent content={content} isUser={isUser} />
        </div>

        {/* AI 操作按钮 */}
        {!isUser && actions && actions.length > 0 && (
          <AIActionButtons actions={actions} onClick={onActionClick} />
        )}

        {/* 时间戳 */}
        <span className="text-[10px] text-gray-400 mt-1 px-1">
          {formatTime(timestamp)}
        </span>
      </div>
    </div>
  );
});

ChatMessage.displayName = 'ChatMessage';

export default ChatMessage;

// Markdown 渲染器
function MarkdownContent({ content, isUser }: { content: string; isUser: boolean }) {
  return (
    <ReactMarkdown
      className={`prose prose-sm max-w-none ${isUser ? 'prose-invert' : 'dark:prose-invert'}`}
      components={{
        code({ node, inline, className, children, ...props }) {
          const match = /language-(\w+)/.exec(className || '');
          const language = match ? match[1] : '';

          if (!inline && language) {
            return (
              <SyntaxHighlighter
                style={vscDarkPlus}
                language={language}
                PreTag="div"
                className="rounded-lg my-2 text-xs"
                {...props}
              >
                {String(children).replace(/\n$/, '')}
              </SyntaxHighlighter>
            );
          }

          return (
            <code className="bg-gray-100 dark:bg-gray-800 px-1.5 py-0.5 rounded text-xs font-mono" {...props}>
              {children}
            </code>
          );
        },
        // 自定义其他 Markdown 元素样式
        p: ({ children }) => <p className="mb-2 last:mb-0 leading-relaxed">{children}</p>,
        ul: ({ children }) => <ul className="list-disc pl-4 mb-2">{children}</ul>,
        ol: ({ children }) => <ol className="list-decimal pl-4 mb-2">{children}</ol>,
        li: ({ children }) => <li className="mb-0.5">{children}</li>,
        a: ({ children, href }) => (
          <a href={href} className="text-blue-500 hover:underline" target="_blank" rel="noopener">
            {children}
          </a>
        ),
      }}
    >
      {content}
    </ReactMarkdown>
  );
}

function formatTime(date: Date): string {
  return new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
  }).format(date);
}
```

### 5.4.4 AI 操作按钮组件

```typescript
// features/chat/AIActionButtons.tsx
import { memo } from 'react';
import { Button } from '@/components/ui/button';
import { Eye, Play, TestTube, Wrench } from 'lucide-react';
import { AIAction } from '@/types/ai';

interface AIActionButtonsProps {
  actions: AIAction[];
  onClick: (action: AIAction) => void;
}

const iconMap = {
  view_workflow: Eye,
  mock_test: TestTube,
  run_workflow: Play,
  ask_parameter: Wrench,
};

const labelMap = {
  view_workflow: '查看工作流',
  mock_test: 'Mock 测试',
  run_workflow: '立即运行',
  ask_parameter: '补充参数',
};

const AIActionButtons = memo(({ actions, onClick }: AIActionButtonsProps) => {
  return (
    <div className="flex flex-wrap gap-2 mt-2">
      {actions.map((action, index) => {
        const Icon = iconMap[action.type] || Wrench;
        const label = action.label || labelMap[action.type] || action.type;

        return (
          <Button
            key={index}
            variant="outline"
            size="sm"
            className="h-7 text-xs gap-1.5 bg-white dark:bg-gray-800 hover:bg-blue-50 dark:hover:bg-blue-900/20 hover:border-blue-300"
            onClick={() => onClick(action)}
          >
            <Icon size={14} />
            {label}
          </Button>
        );
      })}
    </div>
  );
});

AIActionButtons.displayName = 'AIActionButtons';

export default AIActionButtons;
```

### 5.4.5 流式文本渲染组件

```typescript
// features/chat/StreamingText.tsx
import { memo } from 'react';
import { Bot } from 'lucide-react';
import ReactMarkdown from 'react-markdown';

interface StreamingTextProps {
  content: string;
}

const StreamingText = memo(({ content }: StreamingTextProps) => {
  return (
    <div className="flex gap-3">
      <div className="w-8 h-8 rounded-full bg-gray-200 dark:bg-gray-700 flex items-center justify-center flex-shrink-0">
        <Bot size={16} />
      </div>
      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-2xl rounded-tl-sm px-4 py-2.5 shadow-sm max-w-[80%]">
        <ReactMarkdown
          className="prose prose-sm max-w-none dark:prose-invert"
          components={{
            code: ({ inline, children }) => (
              <code className="bg-gray-100 dark:bg-gray-800 px-1.5 py-0.5 rounded text-xs font-mono">
                {children}
              </code>
            ),
          }}
        >
          {content}
        </ReactMarkdown>
        {/* 光标闪烁效果 */}
        <span className="inline-block w-0.5 h-4 bg-blue-500 ml-0.5 animate-pulse" />
      </div>
    </div>
  );
});

StreamingText.displayName = 'StreamingText';

export default StreamingText;
```

### 5.4.6 输入框组件

```typescript
// features/chat/ChatInput.tsx
import { useState, useRef, KeyboardEvent } from 'react';
import { Textarea } from '@/components/ui/textarea';
import { Button } from '@/components/ui/button';
import { Send, Loader2 } from 'lucide-react';

interface ChatInputProps {
  onSend: (message: string) => void;
  disabled?: boolean;
  placeholder?: string;
}

export default function ChatInput({
  onSend,
  disabled = false,
  placeholder = '输入消息...',
}: ChatInputProps) {
  const [input, setInput] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const handleSend = () => {
    if (!input.trim() || disabled) return;
    onSend(input.trim());
    setInput('');
    // 重置高度
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  // 自动调整高度
  const handleInput = () => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      textareaRef.current.style.height = `${Math.min(
        textareaRef.current.scrollHeight,
        200
      )}px`;
    }
  };

  return (
    <div className="flex gap-2 items-end">
      <Textarea
        ref={textareaRef}
        value={input}
        onChange={(e) => setInput(e.target.value)}
        onKeyDown={handleKeyDown}
        onInput={handleInput}
        placeholder={placeholder}
        disabled={disabled}
        className="min-h-[44px] max-h-[200px] resize-none py-3 pr-4"
        rows={1}
      />
      <Button
        onClick={handleSend}
        disabled={disabled || !input.trim()}
        size="icon"
        className="h-11 w-11 flex-shrink-0"
      >
        {disabled ? (
          <Loader2 size={18} className="animate-spin" />
        ) : (
          <Send size={18} />
        )}
      </Button>
    </div>
  );
}
```

---

## 5.5 聊天上下文管理（详细方案）

### 5.5.1 架构设计

聊天上下文管理采用**四层架构**：

```
┌─────────────────────────────────────────────────────────────┐
│                    应用层 (ChatPanel)                        │
│              调用 sendMessage() → 触发流程                   │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                  上下文组装层 (ContextAssembler)             │
│  1. 获取当前会话消息历史                                     │
│  2. 获取当前页面上下文（选中工作流/节点）                    │
│  3. 获取可用节点列表                                         │
│  4. 组装完整上下文对象                                       │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                压缩管理层 (ContextCompressor)                │
│  1. 估算 Token 数量                                          │
│  2. 判断是否需要压缩                                         │
│  3. 执行压缩策略（滑动窗口/重要性评分/摘要）                │
│  4. 返回压缩后的上下文                                       │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                  缓存管理层 (ChatCache)                      │
│  1. 检查缓存命中                                             │
│  2. 读取/写入 IndexedDB                                     │
│  3. LRU 内存缓存                                             │
│  4. 缓存失效与更新                                           │
└─────────────────────────────────────────────────────────────┘
```

### 5.5.2 上下文结构定义

```typescript
// types/ai.ts

/**
 * 聊天消息
 */
export interface ChatMessage {
  id: string;                    // 唯一标识 (UUID)
  role: 'user' | 'assistant' | 'system';
  content: string;               // 消息内容
  timestamp: Date;
  metadata?: MessageMetadata;
}

/**
 * 消息元数据
 */
export interface MessageMetadata {
  // AI 生成时附带的操作按钮
  actions?: AIAction[];
  
  // 消息关联的业务对象
  contextRef?: {
    type: 'node' | 'workflow' | 'execution';
    id: number;
  };
  
  // Token 消耗（仅 assistant 消息）
  tokenUsage?: {
    input: number;
    output: number;
  };
  
  // 消息重要性评分（用于压缩）
  importanceScore?: number;
  
  // 是否为压缩后的摘要消息
  isSummary?: boolean;
}

/**
 * AI 操作按钮
 */
export interface AIAction {
  type: 'view_workflow' | 'mock_test' | 'run_workflow' | 'ask_parameter' | 'custom';
  label: string;
  payload?: Record<string, any>;
}

/**
 * 当前页面上下文
 */
export interface PageContext {
  currentPage: string;           // 当前路由
  selectedWorkflow?: {           // 当前选中的工作流
    id: number;
    name: string;
    status: string;
  };
  selectedNode?: {               // 当前选中的节点
    id: number;
    name: string;
    language: string;
  };
  recentExecution?: {            // 最近的执行记录
    id: number;
    status: string;
    duration: number;
  };
}

/**
 * 完整上下文对象（发送给 AI）
 */
export interface AIContext {
  // 消息历史
  messages: ChatMessage[];
  
  // 系统提示词
  systemPrompt: string;
  
  // 页面上下文
  pageContext: PageContext;
  
  // 可用节点列表
  availableNodes: AvailableNode[];
  
  // 当前会话信息
  sessionInfo: {
    id: string;
    title: string;
    messageCount: number;
  };
}

export interface AvailableNode {
  name: string;
  description: string;
  language: string;
  parameters: Array<{
    name: string;
    type: string;
    required: boolean;
  }>;
}
```

### 5.5.3 上下文组装服务

```typescript
// services/chatContext.ts
import { ChatMessage, AIContext, PageContext, AvailableNode } from '@/types/ai';
import { useChatStore } from '@/stores/chatStore';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useNodeStore } from '@/stores/nodeStore';
import { useExecutionStore } from '@/stores/executionStore';

const SYSTEM_PROMPT = `你是 FlowX Studio 的 AI 助手，一个帮助用户创建和管理工作流的智能助手。

## 你的能力
1. 理解用户的自动化需求
2. 生成可复用的工作流节点（Python/Go/Bash 等）
3. 编排复杂的工作流（DAG 图）
4. 诊断和修复工作流错误
5. 解释节点参数和代码逻辑

## 工作流规范
- 使用 FlowX YAML 格式定义工作流
- 图结构使用 Mermaid stateDiagram-v2 语法
- 节点通过环境变量接收参数（FLOWX_PARAM_* 格式）
- 节点输出 JSON 到 stdout

## 交互原则
1. 主动询问缺失的关键信息
2. 提供清晰的操作建议
3. 如果用户描述模糊，给出示例帮助澄清
4. 保持对话简洁，聚焦工作流主题
5. 生成节点时必须同时提供 Mock 测试代码`;

/**
 * 组装发送给 AI 的完整上下文
 */
export function assembleAIContext(
  sessionId: string,
  newUserMessage?: string
): AIContext {
  const chatStore = useChatStore.getState();
  const workflowStore = useWorkflowStore.getState();
  const nodeStore = useNodeStore.getState();
  const executionStore = useExecutionStore.getState();

  const session = chatStore.sessions.find((s) => s.id === sessionId);
  if (!session) {
    throw new Error(`Session ${sessionId} not found`);
  }

  // 1. 获取消息历史
  let messages = [...session.messages];
  if (newUserMessage) {
    messages.push({
      id: generateId(),
      role: 'user',
      content: newUserMessage,
      timestamp: new Date(),
    });
  }

  // 2. 构建页面上下文
  const pageContext: PageContext = {
    currentPage: window.location.pathname,
    selectedWorkflow: workflowStore.currentWorkflow
      ? {
          id: workflowStore.currentWorkflow.id,
          name: workflowStore.currentWorkflow.name,
          status: workflowStore.currentWorkflow.status,
        }
      : undefined,
    selectedNode: nodeStore.currentNode
      ? {
          id: nodeStore.currentNode.id,
          name: nodeStore.currentNode.name,
          language: nodeStore.currentNode.language,
        }
      : undefined,
    recentExecution: executionStore.lastExecution
      ? {
          id: executionStore.lastExecution.id,
          status: executionStore.lastExecution.status,
          duration: executionStore.lastExecution.durationMs,
        }
      : undefined,
  };

  // 3. 获取可用节点列表
  const availableNodes: AvailableNode[] = nodeStore.nodes.map((node) => ({
    name: node.name,
    description: node.description,
    language: node.language,
    parameters: node.parameters.map((p) => ({
      name: p.name,
      type: p.type,
      required: p.required,
    })),
  }));

  // 4. 组装完整上下文
  const context: AIContext = {
    messages,
    systemPrompt: SYSTEM_PROMPT,
    pageContext,
    availableNodes,
    sessionInfo: {
      id: session.id,
      title: session.title,
      messageCount: messages.length,
    },
  };

  return context;
}

function generateId(): string {
  return `msg_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
}
```

### 5.5.4 Token 估算器

```typescript
// utils/tokenEstimator.ts

/**
 * 简化的 Token 估算器
 * 
 * 原理：基于字符数的近似估算
 * - 英文：~4 字符/token
 * - 中文：~1.5 字符/token
 * - 代码：~3.5 字符/token
 * 
 * 生产环境建议接入 tiktoken（OpenAI 官方）或调用后端估算
 */
export function estimateTokens(text: string): number {
  if (!text) return 0;

  let tokenCount = 0;
  let i = 0;

  while (i < text.length) {
    const char = text[i];
    const code = char.charCodeAt(0);

    // 中文字符（CJK）
    if (code >= 0x4e00 && code <= 0x9fff) {
      tokenCount += 1;
      i++;
    }
    // 英文单词（连续的字母）
    else if (/[a-zA-Z]/.test(char)) {
      let wordLength = 0;
      while (i < text.length && /[a-zA-Z]/.test(text[i])) {
        wordLength++;
        i++;
      }
      // 英文单词平均 0.75 tokens/字符
      tokenCount += Math.ceil(wordLength * 0.75);
    }
    // 数字
    else if (/\d/.test(char)) {
      let numLength = 0;
      while (i < text.length && /\d/.test(text[i])) {
        numLength++;
        i++;
      }
      tokenCount += Math.ceil(numLength * 0.5);
    }
    // 空白字符和标点
    else {
      tokenCount += 0.25; // 标点符号通常一个 token 包含多个
      i++;
    }
  }

  return Math.ceil(tokenCount);
}

/**
 * 估算消息列表的总 Token 数
 */
export function estimateMessageTokens(messages: Array<{ role: string; content: string }>): number {
  // 每条消息的固定开销（角色标记等）
  const MESSAGE_OVERHEAD = 4;

  return messages.reduce((total, msg) => {
    return total + MESSAGE_OVERHEAD + estimateTokens(msg.content);
  }, 0);
}

/**
 * 估算上下文的 Token 数
 */
export function estimateContextTokens(context: {
  systemPrompt: string;
  messages: Array<{ role: string; content: string }>;
  pageContext: any;
  availableNodes: any[];
}): number {
  let total = 0;

  // 系统提示词
  total += estimateTokens(context.systemPrompt);

  // 消息历史
  total += estimateMessageTokens(context.messages);

  // 页面上下文（JSON 序列化后估算）
  total += estimateTokens(JSON.stringify(context.pageContext));

  // 可用节点列表
  total += estimateTokens(JSON.stringify(context.availableNodes));

  return total;
}

// Token 限制常量
export const TOKEN_LIMITS = {
  GPT4: 8192,           // GPT-4 上下文限制
  GPT4_32K: 32768,      // GPT-4 32K
  GPT35: 4096,          // GPT-3.5
  CLAUDE: 100000,       // Claude
  CLAUDE_INSTANT: 100000,
  LOCAL: 4096,          // 本地模型默认限制
};

// 保留 Token 数（给 AI 回复预留空间）
export const RESPONSE_RESERVED_TOKENS = 1500;
```

### 5.5.5 上下文压缩器

```typescript
// features/chat/contextCompressor.ts
import {
  ChatMessage,
  AIContext,
  MessageMetadata,
} from '@/types/ai';
import {
  estimateContextTokens,
  estimateTokens,
  TOKEN_LIMITS,
  RESPONSE_RESERVED_TOKENS,
} from '@/utils/tokenEstimator';

interface CompressionResult {
  messages: ChatMessage[];
  compressionInfo: {
    originalTokens: number;
    compressedTokens: number;
    removedCount: number;
    summaryCount: number;
    strategy: string;
  };
}

/**
 * 上下文压缩器
 * 
 * 策略优先级：
 * 1. 保留最近 N 条消息（滑动窗口）
 * 2. 移除低重要性消息
 * 3. 对早期消息生成摘要
 */
export function compressContext(
  context: AIContext,
  modelLimit: number = TOKEN_LIMITS.GPT4
): CompressionResult {
  const maxInputTokens = modelLimit - RESPONSE_RESERVED_TOKENS;
  const originalTokens = estimateContextTokens(context);

  // 如果未超限，无需压缩
  if (originalTokens <= maxInputTokens) {
    return {
      messages: context.messages,
      compressionInfo: {
        originalTokens,
        compressedTokens: originalTokens,
        removedCount: 0,
        summaryCount: 0,
        strategy: 'none',
      },
    };
  }

  // 策略 1：滑动窗口 - 保留最近的消息
  let messages = applySlidingWindow(context.messages, maxInputTokens);
  let currentTokens = estimateMessageTokens(messages);

  // 策略 2：如果仍超限，移除低重要性消息
  if (currentTokens > maxInputTokens) {
    messages = removeLowImportanceMessages(messages, maxInputTokens);
    currentTokens = estimateMessageTokens(messages);
  }

  // 策略 3：如果仍超限，对早期消息生成摘要
  let summaryCount = 0;
  if (currentTokens > maxInputTokens) {
    const result = summarizeEarlyMessages(messages, maxInputTokens);
    messages = result.messages;
    summaryCount = result.summaryCount;
    currentTokens = estimateMessageTokens(messages);
  }

  return {
    messages,
    compressionInfo: {
      originalTokens,
      compressedTokens: currentTokens,
      removedCount: context.messages.length - messages.length + summaryCount,
      summaryCount,
      strategy: summaryCount > 0 ? 'summary' : 'window+importance',
    },
  };
}

/**
 * 策略 1：滑动窗口
 * 保留最近的消息，直到 Token 数接近限制
 */
function applySlidingWindow(
  messages: ChatMessage[],
  maxTokens: number
): ChatMessage[] {
  // 始终保留：系统提示词（第一条）、最后一条用户消息、最后一条 AI 回复
  const mustKeep = new Set<number>();
  if (messages.length > 0) mustKeep.add(0); // 第一条
  if (messages.length > 1) mustKeep.add(messages.length - 1); // 最后一条
  if (messages.length > 2) mustKeep.add(messages.length - 2); // 倒数第二条

  // 从后向前添加消息，直到接近限制
  const result: ChatMessage[] = [];
  let currentTokens = 0;

  // 先添加必须保留的消息
  for (const idx of mustKeep) {
    const msg = messages[idx];
    const msgTokens = estimateTokens(msg.content) + 4;
    if (currentTokens + msgTokens <= maxTokens) {
      result[idx] = msg;
      currentTokens += msgTokens;
    }
  }

  // 从后向前填充
  for (let i = messages.length - 1; i >= 0; i--) {
    if (mustKeep.has(i)) continue;

    const msg = messages[i];
    const msgTokens = estimateTokens(msg.content) + 4;

    if (currentTokens + msgTokens <= maxTokens * 0.8) { // 预留 20% 缓冲
      result[i] = msg;
      currentTokens += msgTokens;
    } else {
      break;
    }
  }

  // 按原始顺序返回
  return result.filter(Boolean);
}

/**
 * 策略 2：移除低重要性消息
 * 
 * 重要性评分规则：
 * - 包含 YAML 配置的消息：+10
 * - 包含代码的消息：+8
 * - 用户明确的需求描述：+5
 * - 简单的确认/问候：-3
 * - 错误信息：+6
 * - 包含操作按钮的消息：+7
 */
function removeLowImportanceMessages(
  messages: ChatMessage[],
  maxTokens: number
): ChatMessage[] {
  // 计算每条消息的重要性
  const scoredMessages = messages.map((msg, index) => ({
    msg,
    index,
    score: calculateImportanceScore(msg),
  }));

  // 按重要性排序（保留高分）
  scoredMessages.sort((a, b) => b.score - a.score);

  // 保留高分消息直到接近限制
  const result: ChatMessage[] = [];
  let currentTokens = 0;

  for (const { msg } of scoredMessages) {
    const msgTokens = estimateTokens(msg.content) + 4;
    if (currentTokens + msgTokens <= maxTokens * 0.9) {
      result.push(msg);
      currentTokens += msgTokens;
    }
  }

  // 按原始顺序返回
  result.sort((a, b) => {
    const idxA = messages.indexOf(a);
    const idxB = messages.indexOf(b);
    return idxA - idxB;
  });

  return result;
}

/**
 * 计算消息重要性评分
 */
function calculateImportanceScore(message: ChatMessage): number {
  let score = 0;
  const content = message.content.toLowerCase();

  // 基础分：用户消息比 AI 消息更重要
  if (message.role === 'user') score += 3;

  // 包含 YAML 配置
  if (content.includes('yaml') || content.includes('name:') || content.includes('graph:')) {
    score += 10;
  }

  // 包含代码块
  if (content.includes('```')) {
    score += 8;
  }

  // 包含错误信息
  if (content.includes('error') || content.includes('失败') || content.includes('错误')) {
    score += 6;
  }

  // 包含操作按钮（元数据）
  if (message.metadata?.actions && message.metadata.actions.length > 0) {
    score += 7;
  }

  // 用户需求关键词
  const demandKeywords = ['帮我', '需要', '想要', '做一个', '创建', '生成'];
  if (demandKeywords.some((kw) => content.includes(kw))) {
    score += 5;
  }

  // 简单问候（降权）
  const greetingKeywords = ['你好', 'hi', 'hello', '在吗', '谢谢'];
  if (greetingKeywords.some((kw) => content.includes(kw)) && content.length < 20) {
    score -= 3;
  }

  // 消息长度因子（中等长度更重要）
  const length = content.length;
  if (length > 200 && length < 2000) {
    score += 2;
  }

  // 时间衰减（越新的消息越重要）
  const age = Date.now() - message.timestamp.getTime();
  const hoursAgo = age / (1000 * 60 * 60);
  score += Math.max(0, 5 - hoursAgo * 0.5); // 新消息 +5，每小时衰减 0.5

  return score;
}

/**
 * 策略 3：对早期消息生成摘要
 * 
 * 将多条早期消息合并为一条摘要消息
 */
function summarizeEarlyMessages(
  messages: ChatMessage[],
  maxTokens: number
): { messages: ChatMessage[]; summaryCount: number } {
  // 保留最近 6 条消息，对更早的消息生成摘要
  const KEEP_RECENT = 6;
  const recentMessages = messages.slice(-KEEP_RECENT);
  const earlyMessages = messages.slice(0, -KEEP_RECENT);

  if (earlyMessages.length < 3) {
    // 消息太少，不生成摘要
    return { messages, summaryCount: 0 };
  }

  // 生成摘要
  const summary = generateSummary(earlyMessages);
  const summaryMessage: ChatMessage = {
    id: `summary_${Date.now()}`,
    role: 'system',
    content: `[历史对话摘要]\n${summary}`,
    timestamp: new Date(),
    metadata: {
      isSummary: true,
    },
  };

  const result = [summaryMessage, ...recentMessages];
  const currentTokens = estimateMessageTokens(result);

  // 如果仍超限，逐步移除早期摘要中的细节
  if (currentTokens > maxTokens) {
    summaryMessage.content = `[历史对话摘要]\n${generateConciseSummary(earlyMessages)}`;
  }

  return {
    messages: result,
    summaryCount: earlyMessages.length,
  };
}

/**
 * 生成摘要（客户端简化版）
 * 
 * 实际生产环境建议：
 * - 调用后端 AI 服务生成摘要
 * - 或使用更复杂的 NLP 算法
 */
function generateSummary(messages: ChatMessage[]): string {
  const parts: string[] = [];

  // 提取用户需求
  const userDemands = messages
    .filter((m) => m.role === 'user')
    .map((m) => m.content)
    .slice(0, 3);

  if (userDemands.length > 0) {
    parts.push(`用户需求：${userDemands.join('；')}`);
  }

  // 提取已生成的节点
  const nodeMatches = messages
    .flatMap((m) => m.content.match(/节点[名称]*[：:]\s*(\w+)/g) || []);
  if (nodeMatches.length > 0) {
    parts.push(`已生成节点：${[...new Set(nodeMatches)].join(', ')}`);
  }

  // 提取已创建的工作流
  const workflowMatches = messages
    .flatMap((m) => m.content.match(/工作流[名称]*[：:]\s*(\w+)/g) || []);
  if (workflowMatches.length > 0) {
    parts.push(`已创建工作流：${[...new Set(workflowMatches)].join(', ')}`);
  }

  // 提取执行结果
  const successMatches = messages.filter((m) =>
    m.content.includes('执行成功') || m.content.includes('Mock 测试通过')
  );
  if (successMatches.length > 0) {
    parts.push(`执行状态：已成功执行 ${successMatches.length} 次`);
  }

  // 提取错误信息
  const errorMessages = messages
    .filter((m) => m.content.includes('错误') || m.content.includes('失败'))
    .slice(0, 2);
  if (errorMessages.length > 0) {
    parts.push(`需要注意的问题：${errorMessages.map((m) => m.content.substring(0, 50)).join('；')}`);
  }

  return parts.join('\n') || '用户此前已进行工作流相关的对话。';
}

/**
 * 生成精简摘要（Token 紧张时使用）
 */
function generateConciseSummary(messages: ChatMessage[]): string {
  const userCount = messages.filter((m) => m.role === 'user').length;
  const assistantCount = messages.filter((m) => m.role === 'assistant').length;

  return `此前已交换 ${userCount} 条用户需求和 ${assistantCount} 条 AI 回复。对话涉及工作流创建和节点生成。`;
}
```

### 5.5.6 缓存管理服务

```typescript
// services/chatCache.ts
import { get, set, del, keys } from 'idb-keyval';
import { ChatSession, ChatMessage } from '@/types/ai';

// 内存 LRU 缓存
class LRUCache<K, V> {
  private cache: Map<K, V>;
  private maxSize: number;

  constructor(maxSize: number) {
    this.cache = new Map();
    this.maxSize = maxSize;
  }

  get(key: K): V | undefined {
    const value = this.cache.get(key);
    if (value !== undefined) {
      // 访问后移到末尾（最新）
      this.cache.delete(key);
      this.cache.set(key, value);
    }
    return value;
  }

  set(key: K, value: V): void {
    if (this.cache.has(key)) {
      this.cache.delete(key);
    } else if (this.cache.size >= this.maxSize) {
      // 淘汰最旧的
      const firstKey = this.cache.keys().next().value;
      this.cache.delete(firstKey);
    }
    this.cache.set(key, value);
  }

  delete(key: K): void {
    this.cache.delete(key);
  }

  clear(): void {
    this.cache.clear();
  }
}

// 缓存键前缀
const CACHE_PREFIX = 'flowx_chat_';
const SESSION_CACHE_KEY = `${CACHE_PREFIX}sessions`;

// 内存缓存实例（最多缓存 10 个会话）
const memoryCache = new LRUCache<string, ChatSession>(10);

/**
 * 聊天缓存服务
 */
export const chatCache = {
  /**
   * 保存会话到缓存（内存 + IndexedDB）
   */
  async saveSession(session: ChatSession): Promise<void> {
    // 1. 更新内存缓存
    memoryCache.set(session.id, session);

    // 2. 异步写入 IndexedDB
    await set(`${CACHE_PREFIX}session_${session.id}`, session);

    // 3. 更新会话索引
    const sessionIds = await this.getSessionIds();
    if (!sessionIds.includes(session.id)) {
      sessionIds.push(session.id);
      await set(SESSION_CACHE_KEY, sessionIds);
    }
  },

  /**
   * 获取会话（优先内存缓存）
   */
  async getSession(sessionId: string): Promise<ChatSession | undefined> {
    // 1. 检查内存缓存
    const cached = memoryCache.get(sessionId);
    if (cached) return cached;

    // 2. 回退到 IndexedDB
    const session = await get<ChatSession>(`${CACHE_PREFIX}session_${sessionId}`);
    if (session) {
      // 恢复到内存缓存
      memoryCache.set(sessionId, session);
    }
    return session;
  },

  /**
   * 获取所有会话 ID
   */
  async getSessionIds(): Promise<string[]> {
    return (await get<string[]>(SESSION_CACHE_KEY)) || [];
  },

  /**
   * 获取所有会话
   */
  async getAllSessions(): Promise<ChatSession[]> {
    const sessionIds = await this.getSessionIds();
    const sessions = await Promise.all(
      sessionIds.map((id) => this.getSession(id))
    );
    return sessions.filter(Boolean) as ChatSession[];
  },

  /**
   * 删除会话
   */
  async deleteSession(sessionId: string): Promise<void> {
    // 1. 清除内存缓存
    memoryCache.delete(sessionId);

    // 2. 清除 IndexedDB
    await del(`${CACHE_PREFIX}session_${sessionId}`);

    // 3. 更新索引
    const sessionIds = await this.getSessionIds();
    const updated = sessionIds.filter((id) => id !== sessionId);
    await set(SESSION_CACHE_KEY, updated);
  },

  /**
   * 添加消息到会话缓存
   */
  async appendMessage(sessionId: string, message: ChatMessage): Promise<void> {
    const session = await this.getSession(sessionId);
    if (!session) return;

    session.messages.push(message);
    session.updatedAt = new Date();

    await this.saveSession(session);
  },

  /**
   * 清空所有缓存
   */
  async clearAll(): Promise<void> {
    memoryCache.clear();

    const sessionIds = await this.getSessionIds();
    await Promise.all(
      sessionIds.map((id) => del(`${CACHE_PREFIX}session_${id}`))
    );
    await del(SESSION_CACHE_KEY);
  },

  /**
   * 缓存预热：启动时加载最近会话到内存
   */
  async warmUp(): Promise<void> {
    const sessionIds = await this.getSessionIds();
    // 只加载最近 3 个会话到内存
    const recentIds = sessionIds.slice(-3);
    await Promise.all(
      recentIds.map((id) => this.getSession(id))
    );
  },

  /**
   * 缓存统计
   */
  async getStats(): Promise<{
    memoryCached: number;
    diskCached: number;
    totalMessages: number;
  }> {
    const sessions = await this.getAllSessions();
    return {
      memoryCached: memoryCache['cache'].size,
      diskCached: sessions.length,
      totalMessages: sessions.reduce(
        (sum, s) => sum + s.messages.length,
        0
      ),
    };
  },
};

/**
 * 缓存优化 Hook
 */
export function useChatCache() {
  return {
    saveSession: chatCache.saveSession.bind(chatCache),
    getSession: chatCache.getSession.bind(chatCache),
    deleteSession: chatCache.deleteSession.bind(chatCache),
    appendMessage: chatCache.appendMessage.bind(chatCache),
    getAllSessions: chatCache.getAllSessions.bind(chatCache),
    clearAll: chatCache.clearAll.bind(chatCache),
    warmUp: chatCache.warmUp.bind(chatCache),
    getStats: chatCache.getStats.bind(chatCache),
  };
}
```

### 5.5.7 聊天状态管理（Zustand）

```typescript
// stores/chatStore.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { ChatSession, ChatMessage, AIAction, AIContext } from '@/types/ai';
import { chatCache } from '@/services/chatCache';
import { compressContext } from '@/features/chat/contextCompressor';
import { assembleAIContext } from '@/services/chatContext';
import { api } from '@/api/client';

interface ChatStore {
  // 状态
  sessions: ChatSession[];
  activeSessionId: string | null;
  isGenerating: boolean;
  streamingContent: string;

  // 操作
  createSession: () => string;
  setActiveSession: (sessionId: string) => void;
  sendMessage: (content: string) => Promise<void>;
  appendStreamingContent: (content: string) => void;
  finalizeStreaming: () => void;
  executeAction: (action: AIAction) => void;
  loadSessions: () => Promise<void>;
}

export const useChatStore = create<ChatStore>()(
  persist(
    (set, get) => ({
      sessions: [],
      activeSessionId: null,
      isGenerating: false,
      streamingContent: '',

      createSession: () => {
        const session: ChatSession = {
          id: `sess_${Date.now()}`,
          title: '新对话',
          messages: [],
          createdAt: new Date(),
          updatedAt: new Date(),
        };

        set((state) => ({
          sessions: [...state.sessions, session],
          activeSessionId: session.id,
        }));

        // 异步保存到缓存
        chatCache.saveSession(session);

        return session.id;
      },

      setActiveSession: (sessionId: string) => {
        set({ activeSessionId: sessionId });
      },

      sendMessage: async (content: string) => {
        const { activeSessionId, sessions } = get();
        if (!activeSessionId) return;

        const session = sessions.find((s) => s.id === activeSessionId);
        if (!session) return;

        // 1. 添加用户消息到会话
        const userMessage: ChatMessage = {
          id: `msg_${Date.now()}`,
          role: 'user',
          content,
          timestamp: new Date(),
        };

        const updatedMessages = [...session.messages, userMessage];
        const updatedSession = {
          ...session,
          messages: updatedMessages,
          updatedAt: new Date(),
        };

        set((state) => ({
          sessions: state.sessions.map((s) =>
            s.id === activeSessionId ? updatedSession : s
          ),
          isGenerating: true,
          streamingContent: '',
        }));

        // 保存到缓存
        await chatCache.saveSession(updatedSession);

        try {
          // 2. 组装完整上下文
          const context = assembleAIContext(activeSessionId, content);

          // 3. 压缩上下文（防止超长）
          const { messages: compressedMessages, compressionInfo } =
            compressContext(context);

          if (compressionInfo.strategy !== 'none') {
            console.log(
              `Context compressed: ${compressionInfo.originalTokens} → ${compressionInfo.compressedTokens} tokens ` +
              `(${compressionInfo.removedCount} messages removed/summarized)`
            );
          }

          // 4. 发送 SSE 请求
          const response = await api.ai.chatStream({
            session_id: activeSessionId,
            messages: compressedMessages.map((m) => ({
              role: m.role,
              content: m.content,
            })),
            context: {
              page_context: context.pageContext,
              available_nodes: context.availableNodes,
            },
          });

          // 5. 处理流式响应
          const reader = response.body?.getReader();
          if (!reader) throw new Error('No response body');

          let fullContent = '';
          const decoder = new TextDecoder();

          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            const chunk = decoder.decode(value);
            const lines = chunk.split('\n');

            for (const line of lines) {
              if (!line.startsWith('data: ')) continue;

              try {
                const data = JSON.parse(line.slice(6));

                if (data.type === 'content') {
                  fullContent += data.content;
                  set({ streamingContent: fullContent });
                } else if (data.type === 'action') {
                  // 处理 AI 操作按钮
                  console.log('AI action:', data.action);
                } else if (data.type === 'complete') {
                  // 流结束
                  break;
                }
              } catch (e) {
                // 忽略解析错误
              }
            }
          }

          // 6. 保存 AI 回复
          const assistantMessage: ChatMessage = {
            id: `msg_${Date.now()}_ai`,
            role: 'assistant',
            content: fullContent,
            timestamp: new Date(),
            metadata: {
              actions: extractActions(fullContent),
            },
          };

          const finalSession = {
            ...updatedSession,
            messages: [...updatedMessages, assistantMessage],
            updatedAt: new Date(),
            // 更新会话标题（基于第一条用户消息）
            title:
              updatedSession.title === '新对话'
                ? content.slice(0, 20) + (content.length > 20 ? '...' : '')
                : updatedSession.title,
          };

          set((state) => ({
            sessions: state.sessions.map((s) =>
              s.id === activeSessionId ? finalSession : s
            ),
            isGenerating: false,
            streamingContent: '',
          }));

          await chatCache.saveSession(finalSession);
        } catch (error) {
          console.error('Chat error:', error);

          // 添加错误消息
          const errorMessage: ChatMessage = {
            id: `msg_${Date.now()}_error`,
            role: 'assistant',
            content: '抱歉，处理您的请求时出现了错误。请稍后重试。',
            timestamp: new Date(),
          };

          set((state) => ({
            sessions: state.sessions.map((s) =>
              s.id === activeSessionId
                ? { ...s, messages: [...s.messages, errorMessage] }
                : s
            ),
            isGenerating: false,
            streamingContent: '',
          }));
        }
      },

      appendStreamingContent: (content: string) => {
        set((state) => ({
          streamingContent: state.streamingContent + content,
        }));
      },

      finalizeStreaming: () => {
        set({ isGenerating: false, streamingContent: '' });
      },

      executeAction: (action: AIAction) => {
        console.log('Executing action:', action);
        // 根据 action.type 触发不同的业务逻辑
        switch (action.type) {
          case 'view_workflow':
            // 跳转到工作流详情
            break;
          case 'mock_test':
            // 触发 Mock 测试
            break;
          case 'run_workflow':
            // 触发工作流执行
            break;
          case 'ask_parameter':
            // 在聊天中请求用户补充参数
            break;
        }
      },

      loadSessions: async () => {
        const sessions = await chatCache.getAllSessions();
        set({ sessions });
      },
    }),
    {
      name: 'flowx-chat-store',
      partialize: (state) => ({
        sessions: state.sessions.slice(-5), // 只持久化最近 5 个会话
        activeSessionId: state.activeSessionId,
      }),
    }
  )
);

/**
 * 从 AI 回复中提取操作按钮
 * 
 * 约定：AI 在回复末尾使用特殊格式返回操作按钮
 * 例如：
 * [ACTION:view_workflow]查看工作流[/ACTION]
 * [ACTION:mock_test]Mock 测试[/ACTION]
 */
function extractActions(content: string): AIAction[] {
  const actions: AIAction[] = [];
  const regex = /\[ACTION:(\w+)\](.+?)\[\/ACTION\]/g;
  let match;

  while ((match = regex.exec(content)) !== null) {
    actions.push({
      type: match[1] as AIAction['type'],
      label: match[2],
    });
  }

  // 从内容中移除动作标记
  return actions;
}
```

### 5.5.8 缓存优化策略总结

| 优化策略 | 实现方式 | 目的 |
|---------|---------|------|
| **内存 LRU 缓存** | `LRUCache` 类，最多 10 个会话 | 减少 IndexedDB 读取次数，提升响应速度 |
| **IndexedDB 持久化** | `idb-keyval` 库 | 页面刷新后对话历史不丢失 |
| **缓存预热** | 启动时加载最近 3 个会话 | 用户打开应用后立即可用 |
| **部分持久化** | Zustand persist 只存最近 5 个会话 | 减少 localStorage 体积 |
| **异步写入** | 先更新内存，后台写入 IndexedDB | 不阻塞 UI 交互 |
| **缓存失效** | 会话删除时同步清理内存+磁盘 | 防止内存泄漏 |

### 5.5.9 上下文压缩策略总结

| 压缩策略 | 触发条件 | 效果 |
|---------|---------|------|
| **滑动窗口** | Token 超过限制的 80% | 保留最近消息，丢弃早期消息 |
| **重要性评分** | 滑动窗口后仍超限 | 基于内容特征保留高分消息 |
| **摘要生成** | 重要性评分后仍超限 | 将早期多条消息合并为一条摘要 |
| **精简摘要** | Token 极度紧张时 | 极端压缩，只保留对话概要 |

**压缩信息反馈**：
每次压缩后，在浏览器控制台输出压缩统计，便于调试：
```
Context compressed: 8500 → 4200 tokens (12 messages removed/summarized)
Strategy: summary
```

---

## 5.6 实时通信设计

### 5.6.1 SSE 连接管理

```typescript
// hooks/useSSE.ts
import { useEffect, useRef, useCallback } from 'react';

interface SSEOptions {
  onMessage?: (data: any) => void;
  onError?: (error: Event) => void;
  onOpen?: () => void;
  onClose?: () => void;
}

export function useSSE(url: string, options: SSEOptions = {}) {
  const eventSourceRef = useRef<EventSource | null>(null);

  const connect = useCallback(() => {
    if (eventSourceRef.current) return;

    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onopen = () => {
      options.onOpen?.();
    };

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        options.onMessage?.(data);
      } catch (e) {
        console.error('SSE parse error:', e);
      }
    };

    es.onerror = (error) => {
      options.onError?.(error);
      // 自动重连
      setTimeout(() => {
        disconnect();
        connect();
      }, 3000);
    };
  }, [url, options]);

  const disconnect = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
      options.onClose?.();
    }
  }, [options]);

  useEffect(() => {
    connect();
    return disconnect;
  }, [connect, disconnect]);

  return { disconnect };
}
```

---

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

---

## 5.8 性能优化策略

1. **虚拟滚动**：节点列表和执行日志使用虚拟滚动，避免大量 DOM 节点
2. **懒加载**：工作流图节点懒加载，首次只渲染可见区域
3. **防抖**：搜索输入使用防抖，减少 API 请求
4. **缓存**：React Query 自动缓存和失效管理
5. **代码分割**：路由级代码分割，按需加载页面组件
6. **构建优化**：Vite 开启 tree-shaking 和压缩
7. **ReactFlow 性能**：
   - 使用 `memo` 包裹自定义节点组件
   - 仅更新状态变化的节点（通过 `setNodes` 的函数式更新）
   - 使用 `onlyRenderVisibleElements` 属性
8. **聊天上下文性能**：
   - Token 估算使用近似算法，避免昂贵计算
   - 压缩操作在 Web Worker 中执行（消息量大时）
   - 缓存操作异步化，不阻塞主线程

---

## 5.9 主题与样式

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
      },
      animation: {
        'dash': 'dash 1s linear infinite',
      },
      keyframes: {
        dash: {
          '0%': { strokeDashoffset: '24' },
          '100%': { strokeDashoffset: '0' },
        },
      },
    }
  }
};
```

暗色模式使用 Tailwind 的 `dark:` 前缀实现。