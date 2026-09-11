package flowx

import (
	"context"
	"fmt"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/dag"
	"github.com/LerkoX/flowx/metadata"
	"gopkg.in/yaml.v2"
)

// setupMetadata 设置流水线的metadata
func (r *RuntimeImpl) setupMetadata(ctx context.Context, workflow dag.Workflow, config *core.WorkflowConfig) error {
	// 检查是否有metadata配置（注意配置中是Metadate）
	// 只有当配置了 Metadate.Type 且有数据时才创建 store
	if config.Metadate.Type == "" || config.Metadate.Data == nil || len(config.Metadate.Data) == 0 {
		return nil
	}

	// 创建metadata store
	factory := metadata.NewMetadataStoreFactory()
	store, err := factory.Create(config.Metadate, workflow.Id())
	if err != nil {
		return fmt.Errorf("failed to create metadata store: %w", err)
	}

	// 设置到workflow
	workflow.SetMetadata(store)
	return nil
}

// parseConfig 解析流水线配置
func (r *RuntimeImpl) parseConfig(config string) (*core.WorkflowConfig, error) {
	// 直接解析为 WorkflowConfig
	// Param 和 Metadate.Data 保持 map[string]interface{}（yaml.v2 兼容）
	var workflowConfig core.WorkflowConfig
	err := yaml.Unmarshal([]byte(config), &workflowConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml config: %w", err)
	}

	return &workflowConfig, nil
}