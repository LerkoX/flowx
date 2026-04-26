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
func (r *RuntimeImpl) setupMetadata(ctx context.Context, pipeline dag.Pipeline, config *core.PipelineConfig) error {
	// 检查是否有metadata配置（注意配置中是Metadate）
	// 只有当配置了 Metadate.Type 且有数据时才创建 store
	if config.Metadate.Type == "" || config.Metadate.Data == nil || len(config.Metadate.Data) == 0 {
		return nil
	}

	// 创建metadata store
	factory := metadata.NewMetadataStoreFactory()
	store, err := factory.Create(config.Metadate, pipeline.Id())
	if err != nil {
		return fmt.Errorf("failed to create metadata store: %w", err)
	}

	// 设置到pipeline
	pipeline.SetMetadata(store)
	return nil
}

// parseConfig 解析流水线配置
func (r *RuntimeImpl) parseConfig(config string) (*core.PipelineConfig, error) {
	// 直接解析为 PipelineConfig
	// Param 和 Metadate.Data 保持 map[string]interface{}（yaml.v2 兼容）
	var pipelineConfig core.PipelineConfig
	err := yaml.Unmarshal([]byte(config), &pipelineConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml config: %w", err)
	}

	return &pipelineConfig, nil
}