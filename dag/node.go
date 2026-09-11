package dag

import "github.com/LerkoX/flowx/core"

type Node interface {
	//ID 获取节点唯一id
	Id() string
	//WorkflowId 获取节点所属的流水线id
	WorkflowId() string
	//Status 获取节点状态
	Status() string
	//Get 获取节点属性数据
	Get(key string) string
	// Set 设置节点属性数据
	Set(key string, value any)
	// GetExecutor 获取节点执行器名称
	GetExecutor() string
	// GetSteps 获取节点执行步骤
	GetSteps() []core.Step
	// GetConfig 获取节点配置
	GetConfig() map[string]any
	// 运行时状态管理
	GetRuntimeStatus() *core.NodeRuntimeStatus
	SetRuntimeStatus(status *core.NodeRuntimeStatus)
	// 步骤状态管理
	GetStepRuntimeStatus(stepName string) *core.StepRuntimeStatus
	SetStepRuntimeStatus(stepStatus *core.StepRuntimeStatus)
	// 初始化ID
	EnsureIds()
}
