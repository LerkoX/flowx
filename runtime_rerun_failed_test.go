package flowx

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/LerkoX/flowx/core"
)

// 续跑（Rerun/continue）失败节点语义测试：
// 仅 SUCCESS 节点跳过；FAILED/CANCELLED 节点重置状态后重跑；
// 收尾状态按节点终态推导，不再无条件 SUCCESS（exec 498 假阳性事故修复）。

// rerunFailedConfig 生成 A(echo 必成功) → B(可脚本化成败) 的两节点链
func rerunFailedConfig(bCmd string) string {
	return fmt.Sprintf(`
Version: "1.0"
Name: rerun-failed-test
Executors:
  local:
    type: local
Graph: |
  stateDiagram-v2
    [*] --> A
    A --> B
    B --> [*]
Nodes:
  A:
    name: A
    executor: local
    steps:
      - name: step1
        run: echo A
  B:
    name: B
    executor: local
    steps:
      - name: step1
        run: %s
`, bCmd)
}

// loadInto 首轮运行（导出快照）→ 模拟重启 → LoadWorkflow 恢复
func loadInto(t *testing.T, ctx context.Context, id, config string) *RuntimeImpl {
	t.Helper()
	rt := NewRuntime(ctx).(*RuntimeImpl)
	exporter := &exportOnFinishListener{rt: rt, id: id}
	_, err := rt.RunAsync(ctx, id, config, exporter)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}
	// 等首轮跑完（成败不限）：通过 exporter 拿快照即已结束
	deadline := time.Now().Add(15 * time.Second)
	for exporter.snapshot == "" && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if exporter.snapshot == "" {
		t.Fatal("snapshot should have been exported on WorkflowFinish")
	}
	rt2 := NewRuntime(ctx).(*RuntimeImpl)
	if _, err := rt2.LoadWorkflow(ctx, id, exporter.snapshot, NewRecordingListener()); err != nil {
		t.Fatalf("LoadWorkflow failed: %v", err)
	}
	return rt2
}

func nodeStatus(t *testing.T, rt *RuntimeImpl, id, nodeID string) string {
	t.Helper()
	wf, err := rt.Get(id)
	if err != nil {
		t.Fatalf("Get workflow: %v", err)
	}
	node, ok := wf.GetGraph().GetNode(nodeID)
	if !ok {
		t.Fatalf("node %s not found", nodeID)
	}
	rs := node.GetRuntimeStatus()
	if rs == nil {
		return ""
	}
	return rs.Status
}

func waitWorkflowDone(t *testing.T, rt *RuntimeImpl, id string) string {
	t.Helper()
	wf, err := rt.Get(id)
	if err != nil {
		t.Fatalf("Get workflow: %v", err)
	}
	select {
	case <-wf.Done():
	case <-time.After(20 * time.Second):
		t.Fatal("workflow did not finish in time")
	}
	return wf.Status()
}

// 场景①：failed 节点在续跑中重跑并转成功（首次必败、重跑必胜的标记文件脚本），
// 最终状态 Success
func TestRuntimeImpl_Rerun_FailedNodeReruns(t *testing.T) {
	ctx := context.Background()
	marker := filepath.Join(t.TempDir(), "b-ran")
	// 首轮：marker 不存在 → 创建并失败；续跑：marker 存在 → 成功
	bCmd := fmt.Sprintf("test -f %s || { touch %s; exit 1; }", marker, marker)
	rt2 := loadInto(t, ctx, "rerun-failed-1", rerunFailedConfig(bCmd))

	if s := nodeStatus(t, rt2, "rerun-failed-1", "B"); s != core.StatusFailed {
		t.Fatalf("node B should be FAILED after first run, got %q", s)
	}
	if err := rt2.Rerun(ctx, "rerun-failed-1"); err != nil {
		t.Fatalf("Rerun failed: %v", err)
	}
	if got := waitWorkflowDone(t, rt2, "rerun-failed-1"); got != core.StatusSuccess {
		t.Fatalf("workflow should be SUCCESS after failed node reruns to success, got %s", got)
	}
	if s := nodeStatus(t, rt2, "rerun-failed-1", "B"); s != core.StatusSuccess {
		t.Fatalf("node B should be SUCCESS after rerun, got %q", s)
	}
}

// 场景②：failed 节点重跑仍失败 → 最终状态 Failed（不再假 Success）
func TestRuntimeImpl_Rerun_FailedStillFails(t *testing.T) {
	ctx := context.Background()
	rt2 := loadInto(t, ctx, "rerun-failed-2", rerunFailedConfig("exit 1"))

	if s := nodeStatus(t, rt2, "rerun-failed-2", "B"); s != core.StatusFailed {
		t.Fatalf("node B should be FAILED after first run, got %q", s)
	}
	if err := rt2.Rerun(ctx, "rerun-failed-2"); err != nil {
		t.Fatalf("Rerun failed: %v", err)
	}
	if got := waitWorkflowDone(t, rt2, "rerun-failed-2"); got != core.StatusFailed {
		t.Fatalf("workflow should stay FAILED when rerun node still fails, got %s", got)
	}
}

// 场景③：UpdateConfig 允许修改 failed 节点（不再报 cannot modify），
// 改成必胜命令后 Rerun → 最终 Success
func TestRuntimeImpl_UpdateConfig_ModifyFailedNode(t *testing.T) {
	ctx := context.Background()
	rt2 := loadInto(t, ctx, "rerun-failed-3", rerunFailedConfig("exit 1"))

	if s := nodeStatus(t, rt2, "rerun-failed-3", "B"); s != core.StatusFailed {
		t.Fatalf("node B should be FAILED after first run, got %q", s)
	}
	if err := rt2.UpdateConfig(ctx, "rerun-failed-3", rerunFailedConfig("echo fixed")); err != nil {
		t.Fatalf("UpdateConfig on failed node should be allowed, got: %v", err)
	}
	if err := rt2.Rerun(ctx, "rerun-failed-3"); err != nil {
		t.Fatalf("Rerun failed: %v", err)
	}
	if got := waitWorkflowDone(t, rt2, "rerun-failed-3"); got != core.StatusSuccess {
		t.Fatalf("workflow should be SUCCESS after fixed node reruns, got %s", got)
	}
}
