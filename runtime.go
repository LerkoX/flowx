package flowx

import (
	"context"

	"github.com/LerkoX/flowx/dag"
	"github.com/LerkoX/flowx/logger"
	"github.com/LerkoX/flowx/template"
)

// Runtime 运行时
type Runtime interface {
	//获取流水线状态
	Get(id string) (dag.Pipeline, error)
	//取消运行中的流水线
	Cancel(ctx context.Context, id string) error
	//执行异步流水线
	RunAsync(ctx context.Context, id string, config string, listener dag.Listener) (dag.Pipeline, error)
	//执行同步流水线
	RunSync(ctx context.Context, id string, config string, listener dag.Listener) (dag.Pipeline, error)
	//移除流水线记录
	Rm(id string)
	//runtime已经执行完成
	Done() chan struct{}
	//通知runtime
	Notify(data interface{}) error
	//反回runtime公共
	Ctx() context.Context
	//停止后台处理
	StopBackground()
	// 启动后台
	StartBackground()
	// 设置日志推送器
	SetPusher(pusher logger.Pusher)
	// 设置模板引擎
	SetTemplateEngine(engine template.TemplateEngine)
	// 获取模板引擎
	GetTemplateEngine() template.TemplateEngine
	// ExportConfig 导出流水线的运行时配置
	// 返回包含当前运行时状态的 YAML 格式配置字符串
	ExportConfig(id string) (string, error)
	// Pause 暂停运行中的流水线
	Pause(ctx context.Context, id string) error
	// Resume 恢复暂停或停止的流水线
	Resume(ctx context.Context, id string) error
	// ModifyGraph 对暂停或停止的流水线执行图修改（原子操作）
	ModifyGraph(ctx context.Context, id string, modifications dag.GraphModifications) error
	// UpdateConfig 通过新的 YAML 配置自动比对差异并更新流水线图
	// 已执行的节点不允许删除或替换，只允许修改尚未运行的节点
	// 除 Nodes 和 Graph 外的其他配置字段不可更新
	UpdateConfig(ctx context.Context, id string, newConfigYAML string) error
	// ListPipelines 列出所有活跃的流水线ID
	ListPipelines() []string
}
