package dag

import (
	"context"
	"sync"
	"testing"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/executor"
	"github.com/LerkoX/flowx/executor/provider"
)

// testRecordingListener is a test-only listener for dag package tests
type testRecordingListener struct {
	mu     sync.Mutex
	events []Event
}

func newTestRecordingListener() *testRecordingListener {
	return &testRecordingListener{
		events: make([]Event, 0),
	}
}

func (l *testRecordingListener) Handle(p Workflow, event Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *testRecordingListener) Events() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]Event, len(l.events))
	copy(result, l.events)
	return result
}

func (l *testRecordingListener) Count(eventType Event) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	count := 0
	for _, e := range l.events {
		if e == eventType {
			count++
		}
	}
	return count
}

// --- WorkflowImpl.Pause / Resume / IsModifiable ---

func TestWorkflowImpl_Pause_NotRunning(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)
	// 默认状态不是 RUNNING
	err := workflow.Pause()
	if err == nil {
		t.Error("Expected error when pausing non-running workflow")
	}
}

func TestWorkflowImpl_Pause_Running(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)
	workflow.mu.Lock()
	workflow.status = core.StatusRunning
	workflow.mu.Unlock()

	err := workflow.Pause()
	if err != nil {
		t.Errorf("Pause() error = %v", err)
	}

	// 验证 status 已变为 PAUSED
	workflow.mu.Lock()
	if workflow.status != core.StatusPaused {
		t.Errorf("status = %q, want %q after Pause()", workflow.status, core.StatusPaused)
	}
	workflow.mu.Unlock()

	// 再次调用 Pause() 应返回错误（因为状态已经是 PAUSED）
	err2 := workflow.Pause()
	if err2 == nil {
		t.Error("Second Pause() should return error when already paused")
	}
}

func TestWorkflowImpl_Pause_DoublePause(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)
	workflow.mu.Lock()
	workflow.status = core.StatusRunning
	workflow.mu.Unlock()

	// 第一次暂停
	err := workflow.Pause()
	if err != nil {
		t.Fatalf("First Pause() error = %v", err)
	}

	// 第二次暂停应返回错误（非幂等，因为状态已变为 PAUSED）
	err2 := workflow.Pause()
	if err2 == nil {
		t.Error("Second Pause() should return error when already paused")
	}

	// 验证 status 仍为 PAUSED
	workflow.mu.Lock()
	if workflow.status != core.StatusPaused {
		t.Errorf("status = %q, want %q after double Pause()", workflow.status, core.StatusPaused)
	}
	workflow.mu.Unlock()
}

func TestWorkflowImpl_Resume_NotPaused(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)
	// 默认状态不是 PAUSED 或 STOPPED
	err := workflow.Resume(context.Background())
	if err == nil {
		t.Error("Expected error when resuming non-paused workflow")
	}
}

func TestWorkflowImpl_Resume_Paused(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)
	workflow.mu.Lock()
	workflow.status = core.StatusPaused
	workflow.mu.Unlock()

	err := workflow.Resume(context.Background())
	if err != nil {
		t.Errorf("Resume() error = %v", err)
	}

	// 验证 status 已恢复为 RUNNING
	workflow.mu.Lock()
	if workflow.status != core.StatusRunning {
		t.Errorf("status = %q, want %q after Resume()", workflow.status, core.StatusRunning)
	}
	workflow.mu.Unlock()

	// 再次调用 Resume() 应返回错误（因为状态已经是 RUNNING）
	err2 := workflow.Resume(context.Background())
	if err2 == nil {
		t.Error("Second Resume() should return error when already running")
	}
}

func TestWorkflowImpl_Resume_DoubleResume(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)
	workflow.mu.Lock()
	workflow.status = core.StatusPaused
	workflow.mu.Unlock()

	// 第一次恢复
	err := workflow.Resume(context.Background())
	if err != nil {
		t.Fatalf("First Resume() error = %v", err)
	}

	// 第二次恢复应返回错误（非幂等，因为状态已变为 RUNNING）
	err2 := workflow.Resume(context.Background())
	if err2 == nil {
		t.Error("Second Resume() should return error when already running")
	}

	// 验证 status 仍为 RUNNING
	workflow.mu.Lock()
	if workflow.status != core.StatusRunning {
		t.Errorf("status = %q, want %q after double Resume()", workflow.status, core.StatusRunning)
	}
	workflow.mu.Unlock()
}

func TestWorkflowImpl_PauseResume_Cycle(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)

	// 第1轮：暂停→恢复
	workflow.mu.Lock()
	workflow.status = core.StatusRunning
	workflow.mu.Unlock()

	err := workflow.Pause()
	if err != nil {
		t.Fatalf("Round 1 Pause() error = %v", err)
	}

	workflow.mu.Lock()
	if workflow.status != core.StatusPaused {
		t.Errorf("Round 1: status = %q, want %q", workflow.status, core.StatusPaused)
	}
	workflow.mu.Unlock()

	err = workflow.Resume(context.Background())
	if err != nil {
		t.Fatalf("Round 1 Resume() error = %v", err)
	}

	workflow.mu.Lock()
	if workflow.status != core.StatusRunning {
		t.Errorf("Round 1: status = %q, want %q after Resume", workflow.status, core.StatusRunning)
	}
	workflow.mu.Unlock()

	// 第2轮：再次暂停→恢复
	workflow.mu.Lock()
	workflow.status = core.StatusRunning
	workflow.mu.Unlock()

	err = workflow.Pause()
	if err != nil {
		t.Fatalf("Round 2 Pause() error = %v", err)
	}

	workflow.mu.Lock()
	if workflow.status != core.StatusPaused {
		t.Errorf("Round 2: status = %q, want %q", workflow.status, core.StatusPaused)
	}
	workflow.mu.Unlock()

	err = workflow.Resume(context.Background())
	if err != nil {
		t.Fatalf("Round 2 Resume() error = %v", err)
	}

	workflow.mu.Lock()
	if workflow.status != core.StatusRunning {
		t.Errorf("Round 2: status = %q, want %q after Resume", workflow.status, core.StatusRunning)
	}
	workflow.mu.Unlock()
}

func TestWorkflowImpl_IsModifiable_Extra(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"cancelled", core.StatusCancelled, true},
		{"success", core.StatusSuccess, true},
		{"paused", core.StatusPaused, true},
		{"stopped", core.StatusStopped, true},
		{"failed", core.StatusFailed, true},
		{"running", core.StatusRunning, false},
		{"unknown", core.StatusUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := NewWorkflow(context.Background()).(*WorkflowImpl)
			workflow.mu.Lock()
			workflow.status = tt.status
			workflow.mu.Unlock()

			got := workflow.IsModifiable()
			if got != tt.want {
				t.Errorf("IsModifiable() with status %q = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

// --- Notify / notifyCurrentStatus ---

func TestWorkflowImpl_Notify_WithListeningFn(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)

	called := false
	workflow.mu.Lock()
	workflow.listening = func(p Workflow) {
		called = true
	}
	workflow.mu.Unlock()

	workflow.Notify()
	if !called {
		t.Error("ListeningFn was not called by Notify()")
	}
}

func TestWorkflowImpl_Notify_WithListener(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)

	rl := newTestRecordingListener()
	workflow.mu.Lock()
	workflow.listener = rl
	workflow.mu.Unlock()

	workflow.Notify()

	count := rl.Count(core.EventWorkflowStatusUpdate)
	if count != 1 {
		t.Errorf("Expected 1 EventWorkflowStatusUpdate, got %d", count)
	}
}

func TestWorkflowImpl_Notify_WithBoth(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)

	listeningCalled := false
	workflow.mu.Lock()
	workflow.listening = func(p Workflow) {
		listeningCalled = true
	}
	workflow.listener = newTestRecordingListener()
	workflow.mu.Unlock()

	workflow.Notify()

	if !listeningCalled {
		t.Error("ListeningFn was not called")
	}
}

// --- handleInputRequest ---

func TestWorkflowImpl_HandleInputRequest_NilEvent(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)
	node := NewDGANode("test", core.StatusUnknown)
	// 不应 panic
	workflow.handleInputRequest(node, nil)
}

func TestWorkflowImpl_HandleInputRequest_NilRequest(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)
	node := NewDGANode("test", core.StatusUnknown)
	// event.Request 为 nil 不应 panic
	workflow.handleInputRequest(node, &executor.InputRequestEvent{})
}

func TestWorkflowImpl_HandleInputRequest_NilRuntimeStatus(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)
	node := NewDGANode("test", core.StatusUnknown)
	// 不设置 RuntimeStatus
	workflow.handleInputRequest(node, &executor.InputRequestEvent{
		Request: &executor.InputRequest{},
	})
	// 不应 panic，且节点状态应保持为 Unknown
	rt := node.GetRuntimeStatus()
	if rt != nil {
		t.Errorf("expected nil runtimeStatus, got %v", rt)
	}
}

func TestWorkflowImpl_HandleInputRequest_Valid(t *testing.T) {
	workflow := NewWorkflow(context.Background()).(*WorkflowImpl)
	node := NewDGANode("test", core.StatusUnknown)
	node.SetRuntimeStatus(&core.NodeRuntimeStatus{
		Id:     "rt-1",
		Status: core.StatusRunning,
	})

	workflow.handleInputRequest(node, &executor.InputRequestEvent{
		StepName: "step1",
		Request: &executor.InputRequest{
			Prompt: "Enter value:",
			Type:   "text",
		},
	})

	rt := node.GetRuntimeStatus()
	if rt.Status != core.StatusPaused {
		t.Errorf("Status = %q, want %q", rt.Status, core.StatusPaused)
	}
	if rt.InputRequest == nil {
		t.Fatal("InputRequest should not be nil")
	}
	if rt.InputRequest.Prompt != "Enter value:" {
		t.Errorf("Prompt = %q, want %q", rt.InputRequest.Prompt, "Enter value:")
	}
	if rt.InputRequest.Type != "text" {
		t.Errorf("Type = %q, want %q", rt.InputRequest.Type, "text")
	}
}

// --- getStepStatusString ---

func TestGetStepStatusString(t *testing.T) {
	tests := []struct {
		name      string
		node      Node
		stepName  string
		want      string
		setupNode func() Node
	}{
		{
			name:     "nil runtime status",
			node:     NewDGANode("n1", core.StatusUnknown),
			stepName: "step1",
			want:     "unknown",
		},
		{
			name:     "step found",
			setupNode: func() Node {
				n := NewDGANode("n2", core.StatusRunning)
				n.SetRuntimeStatus(&core.NodeRuntimeStatus{
					Steps: []core.StepRuntimeStatus{
						{Name: "step1", Status: core.StatusSuccess},
						{Name: "step2", Status: core.StatusFailed},
					},
				})
				return n
			},
			stepName: "step1",
			want:     "SUCCESS",
		},
		{
			name:     "step not found",
			setupNode: func() Node {
				n := NewDGANode("n3", core.StatusRunning)
				n.SetRuntimeStatus(&core.NodeRuntimeStatus{
					Steps: []core.StepRuntimeStatus{
						{Name: "step1", Status: core.StatusSuccess},
					},
				})
				return n
			},
			stepName: "nonexistent",
			want:     "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var node Node
			if tt.setupNode != nil {
				node = tt.setupNode()
			} else {
				node = tt.node
			}
			got := getStepStatusString(node, tt.stepName)
			if got != tt.want {
				t.Errorf("getStepStatusString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- makeTraversalFn ---

func TestWorkflowImpl_MakeTraversalFn(t *testing.T) {
	ctx := context.Background()
	graph := NewDGAGraph()
	node := NewDGANodeWithConfig("Node1", core.StatusUnknown, "local", "", []core.Step{
		{Name: "step1", Run: "echo hello"},
	}, nil)
	graph.AddVertex(node)

	workflow := NewWorkflow(ctx).(*WorkflowImpl)
	workflow.SetGraph(graph)

	// 设置 executor provider
	execProvider := provider.NewProvider()
	execProvider.RegisterExecutor("local", provider.ExecutorConfig{
		Type:   "local",
		Config: map[string]any{},
	})
	workflow.SetExecutorProvider(execProvider)

	fn := workflow.makeTraversalFn(ctx)
	if fn == nil {
		t.Fatal("makeTraversalFn() returned nil")
	}

	err := fn(ctx, node)
	if err != nil {
		t.Errorf("TraversalFn() error = %v", err)
	}

	// 验证节点执行后 runtimeStatus 已设置
	rt := node.GetRuntimeStatus()
	if rt == nil {
		t.Fatal("node.GetRuntimeStatus() should not be nil after execution")
	}
	if rt.Status != core.StatusSuccess && rt.Status != core.StatusFailed {
		t.Errorf("node status = %q, want SUCCESS or FAILED", rt.Status)
	}

	// 清理 executor
	workflow.cleanupExecutors(ctx)
}
