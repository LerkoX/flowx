package flowx

import (
	"context"
	"testing"
	"time"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/dag"
)

// TestRuntimeImpl_LoadWorkflow_Rerun 测试无状态续跑全流程：
// 运行 → ExportConfig 导出快照 → 模拟重启（新 Runtime）→ LoadWorkflow 恢复
// → UpdateConfig 追加节点 → Rerun 增量执行（已完成节点跳过）
func TestRuntimeImpl_LoadWorkflow_Rerun(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)

	config := `
Version: "1.0"
Name: load-rerun-test
Executors:
  local:
    type: local
Graph: |
  stateDiagram-v2
    [*] --> A
    A --> [*]
Nodes:
  A:
    name: A
    executor: local
    steps:
      - name: step1
        run: echo A
`

	// 运行结束时导出快照（RunAsync 完成后实例即被删除，必须在 WorkflowFinish
	// 事件内同步导出——与 studio 的做法一致）
	exporter := &exportOnFinishListener{rt: rt, id: "load-rerun"}
	workflow, err := rt.RunAsync(ctx, "load-rerun", config, exporter)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}
	waitForStatus(t, workflow, core.StatusSuccess, 10*time.Second)

	snapshotYAML := exporter.snapshot
	if snapshotYAML == "" {
		t.Fatal("snapshot should have been exported on WorkflowFinish")
	}

	// 模拟 server 重启：全新的 Runtime，内存中没有实例
	rt2 := NewRuntime(ctx).(*RuntimeImpl)
	if _, err := rt2.Get("load-rerun"); err == nil {
		t.Fatal("new runtime should not have the workflow")
	}

	// 从快照恢复（不运行）
	p2, err := rt2.LoadWorkflow(ctx, "load-rerun", snapshotYAML, NewRecordingListener())
	if err != nil {
		t.Fatalf("LoadWorkflow failed: %v", err)
	}

	// 节点 A 的运行时状态应恢复为 SUCCESS
	nodeA, ok := p2.GetGraph().GetNode("A")
	if !ok {
		t.Fatal("node A should exist after load")
	}
	if rs := nodeA.GetRuntimeStatus(); rs == nil || rs.Status != core.StatusSuccess {
		t.Fatalf("node A runtime status should be restored to SUCCESS, got %+v", rs)
	}
	// 流水线状态应推导为 SUCCESS（可修改）
	if p2.Status() != core.StatusSuccess {
		t.Fatalf("workflow status should be derived as SUCCESS, got %s", p2.Status())
	}

	// 追加节点 B
	newConfig := `
Version: "1.0"
Name: load-rerun-test
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
        run: echo B
`
	if err := rt2.UpdateConfig(ctx, "load-rerun", newConfig); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// 增量续跑
	if err := rt2.Rerun(ctx, "load-rerun"); err != nil {
		t.Fatalf("Rerun failed: %v", err)
	}
	nodeB, _ := p2.GetGraph().GetNode("B")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if rs := nodeB.GetRuntimeStatus(); rs != nil && rs.Status == core.StatusSuccess {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if rs := nodeB.GetRuntimeStatus(); rs == nil || rs.Status != core.StatusSuccess {
		t.Errorf("node B should have run to SUCCESS, got %+v", rs)
	}
	// A 未被重跑（状态来自快照恢复，未被重置）
	if rs := nodeA.GetRuntimeStatus(); rs == nil || rs.Status != core.StatusSuccess {
		t.Errorf("node A should remain SUCCESS, got %+v", rs)
	}

	// 续跑完成后可再次导出快照（包含 B 的状态），形成闭环
	snapshot2, err := rt2.ExportConfig("load-rerun")
	if err != nil {
		t.Fatalf("ExportConfig after rerun failed: %v", err)
	}
	rt3 := NewRuntime(ctx).(*RuntimeImpl)
	p3, err := rt3.LoadWorkflow(ctx, "load-rerun-2", snapshot2, nil)
	if err != nil {
		t.Fatalf("reload from second snapshot failed: %v", err)
	}
	nodeB3, _ := p3.GetGraph().GetNode("B")
	if rs := nodeB3.GetRuntimeStatus(); rs == nil || rs.Status != core.StatusSuccess {
		t.Errorf("node B status should survive second snapshot round-trip, got %+v", rs)
	}
}

// TestRuntimeImpl_LoadWorkflow_RmRelease 测试 Rm 释放 ID 后可同名重建
func TestRuntimeImpl_LoadWorkflow_RmRelease(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)

	config := `
Version: "1.0"
Name: rm-release
Executors:
  local:
    type: local
Graph: |
  stateDiagram-v2
    [*] --> A
    A --> [*]
Nodes:
  A:
    name: A
    executor: local
    steps:
      - name: step1
        run: echo A
`
	if _, err := rt.LoadWorkflow(ctx, "rm-release", config, nil); err != nil {
		t.Fatalf("LoadWorkflow failed: %v", err)
	}
	// 未运行的实例应是可修改状态（STOPPED）
	p, _ := rt.Get("rm-release")
	if !p.IsModifiable() {
		t.Fatalf("loaded workflow should be modifiable, status=%s", p.Status())
	}

	rt.Rm("rm-release")
	if _, err := rt.LoadWorkflow(ctx, "rm-release", config, nil); err != nil {
		t.Fatalf("LoadWorkflow after Rm should succeed, got: %v", err)
	}
}

// TestRuntimeImpl_Rerun_NotFound 测试 Rerun 未加载/不存在的流水线时报错
func TestRuntimeImpl_Rerun_NotFound(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)

	if err := rt.Rerun(ctx, "no-such-workflow"); err == nil {
		t.Error("Rerun on unknown id should fail")
	}
}

func waitForStatus(t *testing.T, p dag.Workflow, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if p.Status() == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for workflow status %s, current: %s", want, p.Status())
}

// exportOnFinishListener 在 WorkflowFinish 事件内同步导出快照
//（RunAsync 的实例在完成回调返回后即被删除，导出必须在此窗口内完成）
type exportOnFinishListener struct {
	rt       *RuntimeImpl
	id       string
	snapshot string
}

func (l *exportOnFinishListener) Handle(p dag.Workflow, event dag.Event) {
	if event == dag.WorkflowFinish {
		if yaml, err := l.rt.ExportConfig(l.id); err == nil {
			l.snapshot = yaml
		}
	}
}

func (l *exportOnFinishListener) Events() []dag.Event {
	return []dag.Event{dag.WorkflowFinish}
}
