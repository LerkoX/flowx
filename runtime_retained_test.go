package flowx

import (
	"context"
	"testing"
	"time"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/dag"
)

// TestRuntimeImpl_RunAsyncRetained_Rerun 测试保留实例的流水线在完成后
// 可通过 ModifyGraph 添加节点并用 Rerun 继续运行（已完成节点跳过）
func TestRuntimeImpl_RunAsyncRetained_Rerun(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)

	config := `
Version: "1.0"
Name: retained-rerun-test
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

	pipeline, err := rt.RunAsyncRetained(ctx, "retained-rerun", config, NewRecordingListener())
	if err != nil {
		t.Fatalf("RunAsyncRetained failed: %v", err)
	}

	// 等待首次运行完成
	waitForStatus(t, pipeline, core.StatusSuccess, 10*time.Second)

	// 完成后实例应仍保留在 runtime 中（RunAsync 会立即删除）
	if _, err := rt.Get("retained-rerun"); err != nil {
		t.Fatalf("retained pipeline should still be registered: %v", err)
	}

	// 修改图：追加节点 B
	mods := dag.GraphModifications{
		AddNodes: []core.NodeConfig{
			{
				Name:     "B",
				Executor: "local",
				Steps:    []core.Step{{Name: "step1", Run: "echo B"}},
			},
		},
		AddGraph: "stateDiagram-v2\n  [*] --> A\n  A --> B\n  B --> [*]",
	}
	if err := rt.ModifyGraph(ctx, "retained-rerun", mods); err != nil {
		t.Fatalf("ModifyGraph failed: %v", err)
	}

	// 继续运行：已完成的 A 应跳过，仅执行 B
	if err := rt.Rerun(ctx, "retained-rerun"); err != nil {
		t.Fatalf("Rerun failed: %v", err)
	}

	graph := pipeline.GetGraph()
	nodeB, _ := graph.GetNode("B")
	// Rerun 是异步的：直接轮询节点 B 直到 SUCCESS（避免流水线状态
	// SUCCESS→RUNNING→SUCCESS 切换的竞争导致提前返回）
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if rs := nodeB.GetRuntimeStatus(); rs != nil && rs.Status == core.StatusSuccess {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	nodeA, _ := graph.GetNode("A")
	if rs := nodeB.GetRuntimeStatus(); rs == nil || rs.Status != core.StatusSuccess {
		t.Errorf("node B should have run to SUCCESS, got %+v", rs)
	}
	// A 的运行时状态来自首次运行，仍为 SUCCESS 且未被重置
	if nodeA.GetRuntimeStatus() == nil || nodeA.GetRuntimeStatus().Status != core.StatusSuccess {
		t.Errorf("node A should remain SUCCESS, got %+v", nodeA.GetRuntimeStatus())
	}

	// 释放保留的实例
	rt.Rm("retained-rerun")
	if _, err := rt.Get("retained-rerun"); err == nil {
		t.Error("pipeline should be removed after Rm")
	}
}

// TestRuntimeImpl_Rerun_NotFound 测试 Rerun 未保留/不存在的流水线时报错
func TestRuntimeImpl_Rerun_NotFound(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx).(*RuntimeImpl)

	if err := rt.Rerun(ctx, "no-such-pipeline"); err == nil {
		t.Error("Rerun on unknown id should fail")
	}
}

func waitForStatus(t *testing.T, p dag.Pipeline, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if p.Status() == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for pipeline status %s, current: %s", want, p.Status())
}
