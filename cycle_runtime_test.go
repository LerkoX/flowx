package flowx

import (
	"context"
	"testing"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/dag"
)

// TestCyclicWorkflow_Execution 测试循环流水线的实际执行
func TestCyclicWorkflow_Execution(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)
	config := loadTestConfig(t, "cyclic_loop.yaml")
	listener := NewRecordingListener()

	workflow, err := runtime.RunSync(ctx, "cyclic-test", config, listener)
	if err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}

	if workflow.Status() != core.StatusSuccess {
		t.Errorf("Expected SUCCESS, got %s", workflow.Status())
	}

	// 验证节点都完成了
	graph := workflow.GetGraph()
	for _, nodeID := range []string{"A", "B", "C", "D"} {
		node, ok := graph.GetNode(nodeID)
		if !ok {
			t.Errorf("Node %s should exist", nodeID)
			continue
		}
		status := node.GetRuntimeStatus()
		if status == nil {
			t.Errorf("Node %s should have runtime status", nodeID)
			continue
		}
		if status.Status != core.StatusSuccess {
			t.Errorf("Node %s should be SUCCESS, got %s", nodeID, status.Status)
		}
	}
}

// TestAcyclicWorkflow_NoRegression 测试无环图不受循环支持影响
func TestAcyclicWorkflow_NoRegression(t *testing.T) {
	ctx := context.Background()
	runtime := NewRuntime(ctx)
	config := loadTestConfig(t, "sync_workflow.yaml")

	workflow, err := runtime.RunSync(ctx, "acyclic-regression-test", config, NewRecordingListener())
	if err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}

	if workflow.Status() != core.StatusSuccess {
		t.Errorf("Expected SUCCESS, got %s", workflow.Status())
	}

	graph := workflow.GetGraph()
	dgaGraph, ok := graph.(*dag.DGAGraph)
	if !ok {
		t.Fatal("Graph should be DGAGraph")
	}

	if dgaGraph.IsCyclic() {
		t.Error("Acyclic graph should not be marked as cyclic")
	}
}
