package flowx

import (
	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/dag"
	"github.com/LerkoX/flowx/metadata"
	"github.com/LerkoX/flowx/template"
)

// ===== Core 重导出 =====

// 状态常量重导出
const (
	StatusRunning   = core.StatusRunning
	StatusFailed    = core.StatusFailed
	StatusSuccess   = core.StatusSuccess
	StatusTerminate = core.StatusTerminate
	StatusPaused    = core.StatusPaused
	StatusUnknown   = core.StatusUnknown
	StatusCancelled = core.StatusCancelled
	StatusStopped   = core.StatusStopped
)

// 事件常量重导出
const (
	EventPipelineInit                = core.EventPipelineInit
	EventPipelineStart               = core.EventPipelineStart
	EventPipelineFinish              = core.EventPipelineFinish
	EventPipelineExecutorPrepare     = core.EventPipelineExecutorPrepare
	EventPipelineExecutorPrepareDone = core.EventPipelineExecutorPrepareDone
	EventPipelineNodeStart           = core.EventPipelineNodeStart
	EventPipelineNodeFinish          = core.EventPipelineNodeFinish
	EventPipelineCancelled           = core.EventPipelineCancelled
	EventPipelineStatusUpdate        = core.EventPipelineStatusUpdate
	EventPipelinePaused              = core.EventPipelinePaused
	EventPipelineResumed             = core.EventPipelineResumed
	EventPipelineGraphModified       = core.EventPipelineGraphModified
)

// 错误重导出
var (
	ErrInvalidGraph       = core.ErrInvalidGraph
	ErrHasCycle           = core.ErrHasCycle
	ErrPipelineRunning    = core.ErrPipelineRunning
	ErrNodeNotFound       = core.ErrNodeNotFound
	ErrEdgeNotFound       = core.ErrEdgeNotFound
	ErrNodeAlreadyRunning = core.ErrNodeAlreadyRunning
	ErrInvalidState       = core.ErrInvalidState
	ErrImmutableField     = core.ErrImmutableField
	ErrNodeAlreadyExecuted = core.ErrNodeAlreadyExecuted
)

// 配置类型重导出
type PipelineConfig = core.PipelineConfig
type MetadataConfig = core.MetadataConfig
type HTTPMetadataConfig = core.HTTPMetadataConfig
type RedisMetadataConfig = core.RedisMetadataConfig
type AIConfig = core.AIConfig
type ExecutorConfig = core.ExecutorConfig
type LoggingConfig = core.LoggingConfig
type NodeConfig = core.NodeConfig
type Step = core.Step
type ExtractConfig = core.ExtractConfig
type StepRuntimeStatus = core.StepRuntimeStatus
type ExecutorRuntimeInfo = core.ExecutorRuntimeInfo
type NodeRuntimeStatus = core.NodeRuntimeStatus
type InputRequestInfo = core.InputRequestInfo

// UUID 工具重导出
var NewUUID = core.NewUUID
var ValidateUUID = core.ValidateUUID
var FormatUUID = core.FormatUUID

// ===== DAG 重导出 =====

type Pipeline = dag.Pipeline
type PipelineImpl = dag.PipelineImpl
type Graph = dag.Graph
type GraphReader = dag.GraphReader
type Node = dag.Node
type Edge = dag.Edge
type Event = dag.Event
type Listener = dag.Listener
type ListeningFn = dag.ListeningFn
type TraversalFn = dag.TraversalFn
type Metadata = dag.Metadata
type DGAGraph = dag.DGAGraph
type DGANode = dag.DGANode
type DGAEdge = dag.DGAEdge
type EvaluationContext = dag.EvaluationContext
type DGAEvaluationContext = dag.DGAEvaluationContext
type GraphModifications = dag.GraphModifications
type EdgeModification = dag.EdgeModification
type EdgeRemoval = dag.EdgeRemoval
type Snapshotter = dag.Snapshotter
type PipelineSnapshotter = dag.PipelineSnapshotter
type OutputExtractor = dag.OutputExtractor
type CodecBlockExtractor = dag.CodecBlockExtractor
type RegexExtractor = dag.RegexExtractor

// DAG 事件变量重导出
var (
	PipelineInit                = dag.PipelineInit
	PipelineStart               = dag.PipelineStart
	PipelineFinish              = dag.PipelineFinish
	PipelineExecutorPrepare     = dag.PipelineExecutorPrepare
	PipelineExecutorPrepareDone = dag.PipelineExecutorPrepareDone
	PipelineNodeStart           = dag.PipelineNodeStart
	PipelineNodeFinish          = dag.PipelineNodeFinish
	PipelinePaused              = dag.PipelinePaused
	PipelineResumed             = dag.PipelineResumed
	PipelineGraphModified       = dag.PipelineGraphModified
)

// DAG 构造函数重导出
var NewDGAGraph = dag.NewDGAGraph
var NewPipeline = dag.NewPipeline
var NewDGANode = dag.NewDGANode
var NewDGANodeWithConfig = dag.NewDGANodeWithConfig
var NewDGAEdge = dag.NewDGAEdge
var NewConditionalEdge = dag.NewConditionalEdge
var NewConditionalEdgeWithEngine = dag.NewConditionalEdgeWithEngine
var NewEvaluationContext = dag.NewEvaluationContext
var NewPipelineSnapshotter = dag.NewPipelineSnapshotter
var NewCodecBlockExtractor = dag.NewCodecBlockExtractor
var NewRegexExtractor = dag.NewRegexExtractor

// ===== Template 重导出 =====

type TemplateEngine = template.TemplateEngine
type Pongo2TemplateEngine = template.Pongo2TemplateEngine
var NewPongo2TemplateEngine = template.NewPongo2TemplateEngine

// ===== Metadata 重导出 =====

type MetadataStore = metadata.MetadataStore
type MetadataStoreFactory = metadata.MetadataStoreFactory
type InConfigMetadataStore = metadata.InConfigMetadataStore
type HTTPMetadataStore = metadata.HTTPMetadataStore
type RedisMetadataStore = metadata.RedisMetadataStore
type DefaultMetadataStoreFactory = metadata.DefaultMetadataStoreFactory

var NewInConfigMetadataStore = metadata.NewInConfigMetadataStore
var NewHTTPMetadataStore = metadata.NewHTTPMetadataStore
var NewRedisMetadataStore = metadata.NewRedisMetadataStore
var NewMetadataStoreFactory = metadata.NewMetadataStoreFactory
