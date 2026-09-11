package core

const (
	// 流水线状态常量
	StatusRunning   = "RUNNING"
	StatusFailed    = "FAILED"
	StatusSuccess   = "SUCCESS"
	StatusTerminate = "ABORTED"
	StatusPaused    = "PAUSED"
	StatusUnknown   = "UNKNOWN"
	StatusCancelled = "CANCELLED"
	StatusStopped   = "STOPPED"

	// 流水线事件常量
	EventWorkflowInit                = "workflow-init"
	EventWorkflowStart               = "workflow-start"
	EventWorkflowFinish              = "workflow-finish"
	EventWorkflowExecutorPrepare     = "workflow-executor-prepare"
	EventWorkflowExecutorPrepareDone = "workflow-executor-prepare-done"
	EventWorkflowNodeStart           = "workflow-node-start"
	EventWorkflowNodeFinish          = "workflow-node-finish"
	EventWorkflowNodeFailed          = "workflow-node-failed"
	EventWorkflowCancelled           = "workflow-cancelled"
	EventWorkflowStatusUpdate        = "workflow-status-update"
	EventWorkflowPaused              = "workflow-paused"
	EventWorkflowResumed             = "workflow-resumed"
	EventWorkflowGraphModified       = "workflow-graph-modified"
)
