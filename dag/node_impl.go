package dag

import (
	"sync"

	"github.com/LerkoX/flowx/core"
	"github.com/spf13/cast"
	"github.com/thoas/go-funk"
)

type DGANode struct {
	mu            sync.RWMutex
	id            string
	state         string
	property      map[string]any
	pipelineId    string
	executor      string
	steps         []core.Step
	image         string
	config        map[string]any
	runtimeStatus *core.NodeRuntimeStatus
}

// NewDGANode creates a new DGANode with the specified id and state, initializing an empty property map.
func NewDGANode(id, state string) *DGANode {
	return &DGANode{
		id:       id,
		state:    state,
		property: map[string]any{},
		steps:    []core.Step{},
		config:   map[string]any{},
	}
}

// NewDGANodeWithConfig creates a new DGANode with full configuration.
func NewDGANodeWithConfig(id, state, executor, image string, steps []core.Step, config map[string]any) *DGANode {
	return &DGANode{
		id:       id,
		state:    state,
		property: map[string]any{},
		executor: executor,
		steps:    steps,
		image:    image,
		config:   config,
	}
}

func (dgaNode *DGANode) Id() string {
	dgaNode.mu.RLock()
	defer dgaNode.mu.RUnlock()
	return dgaNode.id
}

func (dgaNode *DGANode) PipelineId() string {
	dgaNode.mu.RLock()
	defer dgaNode.mu.RUnlock()
	return dgaNode.pipelineId
}

func (dgaNode *DGANode) Status() string {
	dgaNode.mu.RLock()
	defer dgaNode.mu.RUnlock()
	return dgaNode.state
}

func (dgaNode *DGANode) Get(key string) string {
	dgaNode.mu.RLock()
	defer dgaNode.mu.RUnlock()
	return cast.ToString(funk.Get(dgaNode.property, key))
}

func (dgaNode *DGANode) Set(key string, value any) {
	dgaNode.mu.Lock()
	defer dgaNode.mu.Unlock()
	dgaNode.property[key] = value
}

func (dgaNode *DGANode) GetExecutor() string {
	dgaNode.mu.RLock()
	defer dgaNode.mu.RUnlock()
	return dgaNode.executor
}

func (dgaNode *DGANode) GetSteps() []core.Step {
	dgaNode.mu.RLock()
	defer dgaNode.mu.RUnlock()
	steps := make([]core.Step, len(dgaNode.steps))
	copy(steps, dgaNode.steps)
	return steps
}

func (dgaNode *DGANode) GetConfig() map[string]any {
	dgaNode.mu.RLock()
	defer dgaNode.mu.RUnlock()
	config := make(map[string]any, len(dgaNode.config))
	for k, v := range dgaNode.config {
		config[k] = v
	}
	return config
}

// GetRuntimeStatus 获取运行时状态的深拷贝
func (dgaNode *DGANode) GetRuntimeStatus() *core.NodeRuntimeStatus {
	dgaNode.mu.RLock()
	defer dgaNode.mu.RUnlock()
	return copyNodeRuntimeStatus(dgaNode.runtimeStatus)
}

// SetRuntimeStatus 设置运行时状态
func (dgaNode *DGANode) SetRuntimeStatus(status *core.NodeRuntimeStatus) {
	dgaNode.mu.Lock()
	defer dgaNode.mu.Unlock()
	dgaNode.runtimeStatus = copyNodeRuntimeStatus(status)
}

// GetStepRuntimeStatus 获取指定步骤的运行时状态（拷贝）
func (dgaNode *DGANode) GetStepRuntimeStatus(stepName string) *core.StepRuntimeStatus {
	dgaNode.mu.RLock()
	defer dgaNode.mu.RUnlock()
	if dgaNode.runtimeStatus == nil {
		return nil
	}
	for i := range dgaNode.runtimeStatus.Steps {
		if dgaNode.runtimeStatus.Steps[i].Name == stepName {
			step := dgaNode.runtimeStatus.Steps[i]
			return &step
		}
	}
	return nil
}

// SetStepRuntimeStatus 设置步骤运行时状态
func (dgaNode *DGANode) SetStepRuntimeStatus(stepStatus *core.StepRuntimeStatus) {
	if stepStatus == nil {
		return
	}
	dgaNode.mu.Lock()
	defer dgaNode.mu.Unlock()
	if dgaNode.runtimeStatus == nil {
		dgaNode.runtimeStatus = &core.NodeRuntimeStatus{
			Id:     core.NewUUID(),
			Status: core.StatusUnknown,
			Steps:  []core.StepRuntimeStatus{},
		}
	}
	// 查找并更新或添加
	found := false
	for i := range dgaNode.runtimeStatus.Steps {
		if dgaNode.runtimeStatus.Steps[i].Name == stepStatus.Name {
			dgaNode.runtimeStatus.Steps[i] = *stepStatus
			found = true
			break
		}
	}
	if !found {
		dgaNode.runtimeStatus.Steps = append(dgaNode.runtimeStatus.Steps, *stepStatus)
	}
}

// EnsureIds 确保节点和步骤都有ID
func (dgaNode *DGANode) EnsureIds() {
	dgaNode.mu.Lock()
	defer dgaNode.mu.Unlock()
	if dgaNode.runtimeStatus == nil {
		dgaNode.runtimeStatus = &core.NodeRuntimeStatus{
			Id:     core.NewUUID(),
			Status: core.StatusUnknown,
			Steps:  []core.StepRuntimeStatus{},
		}
	} else if dgaNode.runtimeStatus.Id == "" {
		dgaNode.runtimeStatus.Id = core.NewUUID()
	}
	// 确保步骤都有ID
	for i := range dgaNode.steps {
		if dgaNode.steps[i].Id == "" {
			dgaNode.steps[i].Id = core.NewUUID()
		}
	}
}

// copyNodeRuntimeStatus 深拷贝 NodeRuntimeStatus
func copyNodeRuntimeStatus(status *core.NodeRuntimeStatus) *core.NodeRuntimeStatus {
	if status == nil {
		return nil
	}
	stepsCopy := make([]core.StepRuntimeStatus, len(status.Steps))
	copy(stepsCopy, status.Steps)
	var customCopy map[string]interface{}
	if status.Custom != nil {
		customCopy = make(map[string]interface{}, len(status.Custom))
		for k, v := range status.Custom {
			customCopy[k] = v
		}
	}
	var executorCopy *core.ExecutorRuntimeInfo
	if status.Executor != nil {
		infoCopy := make(map[string]interface{}, len(status.Executor.Info))
		for k, v := range status.Executor.Info {
			infoCopy[k] = v
		}
		executorCopy = &core.ExecutorRuntimeInfo{
			Type:       status.Executor.Type,
			InstanceId: status.Executor.InstanceId,
			Status:     status.Executor.Status,
			Info:       infoCopy,
		}
	}
	var inputRequestCopy *core.InputRequestInfo
	if status.InputRequest != nil {
		inputRequestCopy = &core.InputRequestInfo{
			StepName: status.InputRequest.StepName,
			Prompt:   status.InputRequest.Prompt,
			Type:     status.InputRequest.Type,
		}
	}
	return &core.NodeRuntimeStatus{
		Id:           status.Id,
		Status:       status.Status,
		StartTime:    status.StartTime,
		EndTime:      status.EndTime,
		Steps:        stepsCopy,
		Executor:     executorCopy,
		Custom:       customCopy,
		InputRequest: inputRequestCopy,
	}
}
