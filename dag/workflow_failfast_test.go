package dag

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/executor/provider"
	"github.com/LerkoX/flowx/logger"
)

// recordingPusher 记录所有推送日志的 mock pusher
type recordingPusher struct {
	mu      sync.Mutex
	entries []logger.Entry
}

func (r *recordingPusher) Push(_ context.Context, entry logger.Entry) error {
	r.mu.Lock()
	r.entries = append(r.entries, entry)
	r.mu.Unlock()
	return nil
}

func (r *recordingPusher) PushBatch(_ context.Context, entries []logger.Entry) error {
	r.mu.Lock()
	r.entries = append(r.entries, entries...)
	r.mu.Unlock()
	return nil
}

func (r *recordingPusher) Close() error { return nil }

// errorEntries 返回所有 error 级日志
func (r *recordingPusher) errorEntries() []logger.Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []logger.Entry
	for _, e := range r.entries {
		if e.Level == logger.LevelError {
			out = append(out, e)
		}
	}
	return out
}

func newLocalProvider() *provider.Provider {
	p := provider.NewProvider()
	p.RegisterExecutor("local", provider.ExecutorConfig{
		Type:   "local",
		Config: map[string]interface{}{},
	})
	return p
}

// TestParallelFailFast 验证并行执行时一个节点失败会立即取消同层兄弟节点，
// 流水线快速失败，而不是等所有并行节点跑完。
func TestParallelFailFast(t *testing.T) {
	ctx := context.Background()

	graph := NewDGAGraph()
	// fail 节点立即失败；slow 节点若不取消会跑 30 秒
	nodeFail := NewDGANodeWithConfig("fail", core.StatusUnknown, "local", "",
		[]core.Step{{Name: "step1", Run: "exit 1"}}, nil)
	nodeSlow := NewDGANodeWithConfig("slow", core.StatusUnknown, "local", "",
		[]core.Step{{Name: "step1", Run: "sleep 30"}}, nil)
	graph.AddVertex(nodeFail)
	graph.AddVertex(nodeSlow)

	workflow := NewWorkflow(ctx).(*WorkflowImpl)
	workflow.SetGraph(graph)
	workflow.SetExecutorProvider(newLocalProvider())

	start := time.Now()
	err := workflow.Run(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected workflow to fail")
	}
	// 错误应携带失败节点 ID，便于并行场景定位
	if !strings.Contains(err.Error(), "fail") {
		t.Errorf("Error should identify the failed node, got: %v", err)
	}
	// fail-fast：整体耗时应远小于 slow 节点的 30s（留足进程启动/终止开销）
	if elapsed > 15*time.Second {
		t.Errorf("Expected fail-fast cancellation, but run took %v (slow node not cancelled?)", elapsed)
	}

	// slow 节点不应是成功的（被取消或失败均可，取决于取消信号到达时机）
	if rs := nodeSlow.GetRuntimeStatus(); rs != nil && rs.Status == core.StatusSuccess {
		t.Error("Slow node should not have succeeded after sibling failure")
	}

	// 流水线最终状态应为 failed
	if workflow.Status() != core.StatusFailed {
		t.Errorf("Expected workflow status failed, got %s", workflow.Status())
	}
}

// TestFailurePushedToLog 验证节点执行失败会把错误推送到日志流（此前完全静默）。
func TestFailurePushedToLog(t *testing.T) {
	ctx := context.Background()

	graph := NewDGAGraph()
	nodeFail := NewDGANodeWithConfig("failing-node", core.StatusUnknown, "local", "",
		[]core.Step{{Name: "step1", Run: "exit 42"}}, nil)
	graph.AddVertex(nodeFail)

	workflow := NewWorkflow(ctx).(*WorkflowImpl)
	workflow.SetGraph(graph)
	workflow.SetExecutorProvider(newLocalProvider())

	pusher := &recordingPusher{}
	workflow.SetPusher(pusher)

	if err := workflow.Run(ctx); err == nil {
		t.Fatal("Expected workflow to fail")
	}

	errs := pusher.errorEntries()
	if len(errs) == 0 {
		t.Fatal("Expected at least one error-level log entry for the failed step")
	}

	found := false
	for _, e := range errs {
		if e.Node == "failing-node" && strings.Contains(e.Message, "42") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error log mentioning exit code 42 on node failing-node, got: %+v", errs)
	}
}

// TestExecutorAcquireFailurePushedToLog 验证 executor 获取/准备失败也会推送错误日志
//（如 executor 未注册、daemon 不可达，此前只 fmt.Printf 到 server stdout）。
func TestExecutorAcquireFailurePushedToLog(t *testing.T) {
	ctx := context.Background()

	graph := NewDGAGraph()
	nodeNoExec := NewDGANodeWithConfig("no-exec-node", core.StatusUnknown, "missing", "",
		[]core.Step{{Name: "step1", Run: "echo hi"}}, nil)
	graph.AddVertex(nodeNoExec)

	workflow := NewWorkflow(ctx).(*WorkflowImpl)
	workflow.SetGraph(graph)
	workflow.SetExecutorProvider(provider.NewProvider()) // 未注册任何 executor

	pusher := &recordingPusher{}
	workflow.SetPusher(pusher)

	if err := workflow.Run(ctx); err == nil {
		t.Fatal("Expected workflow to fail")
	}

	errs := pusher.errorEntries()
	found := false
	for _, e := range errs {
		if e.Node == "no-exec-node" && strings.Contains(e.Message, "executor") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error log about executor acquisition failure, got: %+v", errs)
	}
}
