package dag

import (
	"context"
	"errors"

	"github.com/LerkoX/flowx/core"
			"github.com/LerkoX/flowx/logger"
	"github.com/LerkoX/flowx/metadata"
	"github.com/LerkoX/flowx/template"
)

// errReadOnly 是 pipelineSnapshot 的写操作返回的错误
var errReadOnly = errors.New("pipeline snapshot is read-only")

// pipelineSnapshot 是 PipelineImpl 的只读快照，用于回调通知
// 它实现了 Pipeline 接口，但所有写方法都会返回 errReadOnly
// 这样可以在 Notify/NotifyEvent 中安全地将 pipeline 状态传递给外部监听器
// 避免回调期间 pipeline 状态被并发修改导致的不一致问题
type pipelineSnapshot struct {
	id          string
	status      string
	graph       Graph
	currentNode Node
	metadata    Metadata
	param       map[string]core.FieldItem
}

// newPipelineSnapshot 从 PipelineImpl 创建只读快照
// 调用者必须持有 p.mu 的读锁或写锁
func newPipelineSnapshot(p *PipelineImpl) *pipelineSnapshot {
	// 直接拷贝 metadata（避免调用 Metadata() 导致死锁）
	metadataCopy := make(Metadata)
	for k, v := range p.metadata {
		metadataCopy[k] = v
	}

	// 拷贝 param
	paramCopy := make(map[string]core.FieldItem)
	for k, v := range p.param {
		paramCopy[k] = v
	}

	return &pipelineSnapshot{
		id:          p.id,
		status:      p.status,
		graph:       p.graph,
		currentNode: p.currentNode,
		metadata:    metadataCopy,
		param:       paramCopy,
	}
}

// Id 返回流水线的ID
func (s *pipelineSnapshot) Id() string {
	return s.id
}

// GetGraph 返回流水线的图结构
func (s *pipelineSnapshot) GetGraph() Graph {
	return s.graph
}

// SetGraph 设置流水线的图结构（只读，panic）
func (s *pipelineSnapshot) SetGraph(graph Graph) {
	panic(errReadOnly)
}

// Status 返回流水线的整体状态
func (s *pipelineSnapshot) Status() string {
	return s.status
}

// SetMetadata 设置元数据（只读，panic）
func (s *pipelineSnapshot) SetMetadata(store metadata.MetadataStore) {
	panic(errReadOnly)
}

// Metadata 获取元数据
func (s *pipelineSnapshot) Metadata() Metadata {
	result := make(Metadata)
	for k, v := range s.metadata {
		result[k] = v
	}
	return result
}

// Listening 设置流水线执行事件监听（只读，panic）
func (s *pipelineSnapshot) Listening(listener Listener) {
	panic(errReadOnly)
}

// Done 返回一个通道，用于通知流水线何时完成（快照中返回 nil）
func (s *pipelineSnapshot) Done() <-chan struct{} {
	return nil
}

// Run 执行流水线（只读，panic）
func (s *pipelineSnapshot) Run(ctx context.Context) error {
	panic(errReadOnly)
}

// Notify 通知（只读，panic）
func (s *pipelineSnapshot) Notify() {
	panic(errReadOnly)
}

// Cancel 取消流水线（只读，panic）
func (s *pipelineSnapshot) Cancel() {
	panic(errReadOnly)
}

// SetExecutorProvider 设置Executor提供者（只读，panic）
func (s *pipelineSnapshot) SetExecutorProvider(provider ExecutorProvider) {
	panic(errReadOnly)
}

// SetTemplateEngine 设置模板引擎（只读，panic）
func (s *pipelineSnapshot) SetTemplateEngine(engine template.TemplateEngine) {
	panic(errReadOnly)
}

// GetTemplateEngine 获取模板引擎（快照中返回 nil）
func (s *pipelineSnapshot) GetTemplateEngine() template.TemplateEngine {
	return nil
}

// SetPusher 设置日志推送器（只读，panic）
func (s *pipelineSnapshot) SetPusher(pusher logger.Pusher) {
	panic(errReadOnly)
}

// Pause 暂停流水线（只读，panic）
func (s *pipelineSnapshot) Pause() error {
	return errReadOnly
}

// Resume 恢复暂停的流水线（只读，panic）
func (s *pipelineSnapshot) Resume(ctx context.Context) error {
	return errReadOnly
}

// IsModifiable 判断当前是否可修改图
func (s *pipelineSnapshot) IsModifiable() bool {
	switch s.status {
	case core.StatusPaused, core.StatusStopped, core.StatusFailed, core.StatusCancelled, core.StatusSuccess:
		return true
	default:
		return false
	}
}

// CurrentNode 返回当前正在执行的节点
func (s *pipelineSnapshot) CurrentNode() Node {
	return s.currentNode
}

// SetParam 设置 param 值（只读，panic）
func (s *pipelineSnapshot) SetParam(param map[string]interface{}) {
	panic(errReadOnly)
}

// GetParam 获取 param 值（只读）
func (s *pipelineSnapshot) GetParam() Metadata {
	result := make(Metadata, len(s.param))
	for k, v := range s.param {
		result[k] = v
	}
	return result
}

// SetMaxLoopIterations 设置循环图最大迭代次数（只读，panic）
func (s *pipelineSnapshot) SetMaxLoopIterations(max int) {
	panic(errReadOnly)
}

// 预检查 pipelineSnapshot 是否实现了 Pipeline 接口
var _ Pipeline = (*pipelineSnapshot)(nil)
