package dag

import (
	"context"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/executor"
	"github.com/LerkoX/flowx/metadata"
	"github.com/LerkoX/flowx/template"
)

var (
	//监听事件
	PipelineInit                Event = core.EventPipelineInit                // 流水线初始化
	PipelineStart               Event = core.EventPipelineStart               // 流水线开始执行
	PipelineFinish              Event = core.EventPipelineFinish              // 流水线完成
	PipelineExecutorPrepare     Event = core.EventPipelineExecutorPrepare     // 流水线执行器开始准备
	PipelineExecutorPrepareDone Event = core.EventPipelineExecutorPrepareDone // 流水线执行器准备完毕
	PipelineNodeStart           Event = core.EventPipelineNodeStart           // 节点开始
	PipelineNodeFinish          Event = core.EventPipelineNodeFinish          // 节点完成
	PipelinePaused              Event = core.EventPipelinePaused              // 流水线暂停
	PipelineResumed             Event = core.EventPipelineResumed             // 流水线恢复
	PipelineGraphModified       Event = core.EventPipelineGraphModified       // 图被修改
)

type TraversalFn func(ctx context.Context, node Node) error

type Graph interface {
	GraphReader
	//AddVertex 添加顶点
	AddVertex(node Node)
	//AddEdge 添加边
	AddEdge(edge Edge) error
	//RemoveVertex 删除节点及其所有关联边
	RemoveVertex(nodeID string) error
	//RemoveEdge 删除指定的边
	RemoveEdge(srcID, destID string) error
	//HasCycle 检查图中是否存在环
	HasCycle() bool
}

type GraphReader interface {
	//Nodes
	Nodes() map[string]Node
	//Edges 返回所有的边
	Edges() []Edge
	//Traversal 遍历图结构
	Traversal(ctx context.Context, evalCtx EvaluationContext, fn TraversalFn) error
	//GetNode 根据节点ID查找节点
	GetNode(nodeID string) (Node, bool)
	//GetEdge 根据源节点和目标节点ID查找边
	GetEdge(srcID, destID string) (Edge, bool)
	//IncomingEdges 返回指向指定节点的所有边
	IncomingEdges(nodeID string) []Edge
	//OutgoingEdges 返回从指定节点出发的所有边
	OutgoingEdges(nodeID string) []Edge
}

// ExecutorProvider Executor提供者接口
// 从executor/core导入
type ExecutorProvider = executor.ExecutorProvider

// 流水线事件
type Event string

// 我们将整个流水线的运行过程中的事件抽象成对应的Event
// 这样我们就能再外部监听Event
type Listener interface {
	// 处理对应的事件将事件发生的对应的流水线和对应的事件作为参数传入
	Handle(p Pipeline, event Event)
	// 获取当前注册的Event
	Events() []Event
}

// PipelineListeningFn 流水线监听函数
type ListeningFn func(p Pipeline)
type Metadata map[string]core.FieldItem

type Pipeline interface {
	//ID 流水线的id
	Id() string
	//GetGraph 返回图结构
	GetGraph() Graph
	//SetGraph 设置图结构
	SetGraph(graph Graph)
	//Status 返回流水线的整体状态
	Status() string
	//SetMetadata 设置元数据
	SetMetadata(store metadata.MetadataStore)
	//Metadata 获取元数据
	Metadata() Metadata
	//Listening 流水线执行事件监听设置
	Listening(listener Listener)
	//Done流水线是否执行完成
	Done() <-chan struct{}
	//Run执行流水线
	Run(ctx context.Context) error
	//Notify 执行的步骤通知流水线
	Notify()
	//Cancel 取消流水线
	Cancel()
	//SetExecutorProvider 设置Executor提供者
	SetExecutorProvider(provider ExecutorProvider)
	//SetTemplateEngine 设置模板引擎
	SetTemplateEngine(engine template.TemplateEngine)
	//GetTemplateEngine 获取模板引擎
	GetTemplateEngine() template.TemplateEngine
	//Pause 暂停流水线，等待当前层执行完成后暂停
	Pause() error
	//Resume 恢复暂停的流水线
	Resume(ctx context.Context) error
	//IsModifiable 判断当前是否可修改图
	IsModifiable() bool
}
