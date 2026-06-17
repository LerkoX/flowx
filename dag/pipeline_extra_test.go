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

func (l *testRecordingListener) Handle(p Pipeline, event Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *testRecordingListener) Events() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return []Event{
		PipelineInit,
		PipelineStart,
		PipelineFinish,
		PipelineNodeStart,
		PipelineNodeFinish,
	}
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

// --- PipelineImpl.Pause / Resume / IsModifiable ---

func TestPipelineImpl_Pause_NotRunning(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	// 默认状态不是 RUNNING
	err := pipeline.Pause()
	if err == nil {
		t.Error("Expected error when pausing non-running pipeline")
	}
}

func TestPipelineImpl_Pause_Running(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	pipeline.mu.Lock()
	pipeline.status = core.StatusRunning
	pipeline.mu.Unlock()

	err := pipeline.Pause()
	if err != nil {
		t.Errorf("Pause() error = %v", err)
	}

	// 验证 paused 标志已设置
	pipeline.pauseMu.Lock()
	if !pipeline.paused {
		t.Error("paused should be true after Pause()")
	}
	pipeline.pauseMu.Unlock()

	// 再次调用 Pause() 不应 panic（幂等）
	err2 := pipeline.Pause()
	if err2 != nil {
		t.Errorf("Second Pause() error = %v", err2)
	}
}

func TestPipelineImpl_Pause_DoublePause(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	pipeline.mu.Lock()
	pipeline.status = core.StatusRunning
	pipeline.mu.Unlock()

	// 第一次暂停
	err := pipeline.Pause()
	if err != nil {
		t.Fatalf("First Pause() error = %v", err)
	}

	// 第二次暂停不应 panic（幂等）
	err2 := pipeline.Pause()
	if err2 != nil {
		t.Errorf("Second Pause() error = %v", err2)
	}

	// 验证 paused 标志仍为 true
	pipeline.pauseMu.Lock()
	if !pipeline.paused {
		t.Error("paused should still be true after double Pause()")
	}
	pipeline.pauseMu.Unlock()
}

func TestPipelineImpl_Resume_NotPaused(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	// 默认状态不是 PAUSED 或 STOPPED
	err := pipeline.Resume(context.Background())
	if err == nil {
		t.Error("Expected error when resuming non-paused pipeline")
	}
}

func TestPipelineImpl_Resume_Paused(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	pipeline.mu.Lock()
	pipeline.status = core.StatusPaused
	pipeline.mu.Unlock()

	// 先设置 paused 标志
	pipeline.pauseMu.Lock()
	pipeline.paused = true
	pipeline.pauseMu.Unlock()

	err := pipeline.Resume(context.Background())
	if err != nil {
		t.Errorf("Resume() error = %v", err)
	}

	// 验证 paused 标志已清除
	pipeline.pauseMu.Lock()
	if pipeline.paused {
		t.Error("paused should be false after Resume()")
	}
	pipeline.pauseMu.Unlock()

	// 再次调用 Resume() 不应 panic（幂等）
	err2 := pipeline.Resume(context.Background())
	if err2 != nil {
		t.Errorf("Second Resume() error = %v", err2)
	}
}

func TestPipelineImpl_Resume_DoubleResume(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	pipeline.mu.Lock()
	pipeline.status = core.StatusPaused
	pipeline.mu.Unlock()

	// 先设置 paused 标志
	pipeline.pauseMu.Lock()
	pipeline.paused = true
	pipeline.pauseMu.Unlock()

	// 第一次恢复
	err := pipeline.Resume(context.Background())
	if err != nil {
		t.Fatalf("First Resume() error = %v", err)
	}

	// 第二次恢复不应 panic（幂等）
	err2 := pipeline.Resume(context.Background())
	if err2 != nil {
		t.Errorf("Second Resume() error = %v", err2)
	}

	// 验证 paused 标志仍为 false
	pipeline.pauseMu.Lock()
	if pipeline.paused {
		t.Error("paused should still be false after double Resume()")
	}
	pipeline.pauseMu.Unlock()
}

func TestPipelineImpl_PauseResume_Cycle(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)

	// 第1轮：暂停→恢复
	pipeline.mu.Lock()
	pipeline.status = core.StatusRunning
	pipeline.mu.Unlock()

	err := pipeline.Pause()
	if err != nil {
		t.Fatalf("Round 1 Pause() error = %v", err)
	}

	pipeline.pauseMu.Lock()
	if !pipeline.paused {
		t.Error("Round 1: paused should be true")
	}
	pipeline.pauseMu.Unlock()

	pipeline.mu.Lock()
	pipeline.status = core.StatusPaused
	pipeline.mu.Unlock()

	err = pipeline.Resume(context.Background())
	if err != nil {
		t.Fatalf("Round 1 Resume() error = %v", err)
	}

	pipeline.pauseMu.Lock()
	if pipeline.paused {
		t.Error("Round 1: paused should be false after Resume")
	}
	pipeline.pauseMu.Unlock()

	// 第2轮：再次暂停→恢复
	pipeline.mu.Lock()
	pipeline.status = core.StatusRunning
	pipeline.mu.Unlock()

	err = pipeline.Pause()
	if err != nil {
		t.Fatalf("Round 2 Pause() error = %v", err)
	}

	pipeline.pauseMu.Lock()
	if !pipeline.paused {
		t.Error("Round 2: paused should be true")
	}
	pipeline.pauseMu.Unlock()

	pipeline.mu.Lock()
	pipeline.status = core.StatusPaused
	pipeline.mu.Unlock()

	err = pipeline.Resume(context.Background())
	if err != nil {
		t.Fatalf("Round 2 Resume() error = %v", err)
	}

	pipeline.pauseMu.Lock()
	if pipeline.paused {
		t.Error("Round 2: paused should be false after Resume")
	}
	pipeline.pauseMu.Unlock()
}

func TestPipelineImpl_IsModifiable_Extra(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"cancelled", core.StatusCancelled, true},
		{"success", core.StatusSuccess, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := NewPipeline(context.Background()).(*PipelineImpl)
			pipeline.mu.Lock()
			pipeline.status = tt.status
			pipeline.mu.Unlock()

			got := pipeline.IsModifiable()
			if got != tt.want {
				t.Errorf("IsModifiable() with status %q = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

// --- Notify / notifyCurrentStatus ---

func TestPipelineImpl_Notify_WithListeningFn(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)

	called := false
	pipeline.mu.Lock()
	pipeline.listening = func(p Pipeline) {
		called = true
	}
	pipeline.mu.Unlock()

	pipeline.Notify()
	if !called {
		t.Error("ListeningFn was not called by Notify()")
	}
}

func TestPipelineImpl_Notify_WithListener(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)

	rl := newTestRecordingListener()
	pipeline.mu.Lock()
	pipeline.listener = rl
	pipeline.mu.Unlock()

	pipeline.Notify()

	count := rl.Count(core.EventPipelineStatusUpdate)
	if count != 1 {
		t.Errorf("Expected 1 EventPipelineStatusUpdate, got %d", count)
	}
}

func TestPipelineImpl_Notify_WithBoth(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)

	listeningCalled := false
	pipeline.mu.Lock()
	pipeline.listening = func(p Pipeline) {
		listeningCalled = true
	}
	pipeline.listener = newTestRecordingListener()
	pipeline.mu.Unlock()

	pipeline.Notify()

	if !listeningCalled {
		t.Error("ListeningFn was not called")
	}
}

// --- handleInputRequest ---

func TestPipelineImpl_HandleInputRequest_NilEvent(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	node := NewDGANode("test", core.StatusUnknown)
	// 不应 panic
	pipeline.handleInputRequest(node, nil)
}

func TestPipelineImpl_HandleInputRequest_NilRequest(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	node := NewDGANode("test", core.StatusUnknown)
	// event.Request 为 nil 不应 panic
	pipeline.handleInputRequest(node, &executor.InputRequestEvent{})
}

func TestPipelineImpl_HandleInputRequest_NilRuntimeStatus(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	node := NewDGANode("test", core.StatusUnknown)
	// 不设置 RuntimeStatus
	pipeline.handleInputRequest(node, &executor.InputRequestEvent{
		Request: &executor.InputRequest{},
	})
	// 不应 panic
}

func TestPipelineImpl_HandleInputRequest_Valid(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	node := NewDGANode("test", core.StatusUnknown)
	node.SetRuntimeStatus(&core.NodeRuntimeStatus{
		Id:     "rt-1",
		Status: core.StatusRunning,
	})

	pipeline.handleInputRequest(node, &executor.InputRequestEvent{
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

func TestPipelineImpl_MakeTraversalFn(t *testing.T) {
	ctx := context.Background()
	graph := NewDGAGraph()
	node := NewDGANodeWithConfig("Node1", core.StatusUnknown, "local", "", []core.Step{
		{Name: "step1", Run: "echo hello"},
	}, nil)
	graph.AddVertex(node)

	pipeline := NewPipeline(ctx).(*PipelineImpl)
	pipeline.SetGraph(graph)

	// 设置 executor provider
	execProvider := provider.NewProvider()
	execProvider.RegisterExecutor("local", provider.ExecutorConfig{
		Type:   "local",
		Config: map[string]any{},
	})
	pipeline.SetExecutorProvider(execProvider)

	fn := pipeline.makeTraversalFn(ctx)
	if fn == nil {
		t.Fatal("makeTraversalFn() returned nil")
	}

	err := fn(ctx, node)
	if err != nil {
		t.Errorf("TraversalFn() error = %v", err)
	}

	// 清理 executor
	pipeline.cleanupExecutors(ctx)
}
