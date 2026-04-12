package pipelinex

import (
	"context"
	"testing"

	"github.com/LerkoX/pipelinex/executor"
	"github.com/LerkoX/pipelinex/executor/provider"
)

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
	pipeline.status = StatusRunning
	pipeline.pauseChan = make(chan struct{})
	pipeline.mu.Unlock()

	err := pipeline.Pause()
	if err != nil {
		t.Errorf("Pause() error = %v", err)
	}

	// 验证 pauseChan 已关闭
	select {
	case <-pipeline.pauseChan:
		// 正确：channel 已关闭
	default:
		t.Error("pauseChan should be closed after Pause()")
	}
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
	pipeline.status = StatusPaused
	pipeline.resumeChan = make(chan struct{})
	pipeline.mu.Unlock()

	err := pipeline.Resume(context.Background())
	if err != nil {
		t.Errorf("Resume() error = %v", err)
	}

	// 验证 resumeChan 已关闭
	select {
	case <-pipeline.resumeChan:
		// 正确：channel 已关闭
	default:
		t.Error("resumeChan should be closed after Resume()")
	}
}

func TestPipelineImpl_IsModifiable_Extra(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"cancelled", StatusCancelled, true},
		{"success", StatusSuccess, true},
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

	rl := NewRecordingListener()
	pipeline.mu.Lock()
	pipeline.listener = rl
	pipeline.mu.Unlock()

	pipeline.Notify()

	count := rl.Count(EventPipelineStatusUpdate)
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
	pipeline.listener = NewRecordingListener()
	pipeline.mu.Unlock()

	pipeline.Notify()

	if !listeningCalled {
		t.Error("ListeningFn was not called")
	}
}

// --- handleInputRequest ---

func TestPipelineImpl_HandleInputRequest_NilEvent(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	node := NewDGANode("test", StatusUnknown)
	// 不应 panic
	pipeline.handleInputRequest(node, nil)
}

func TestPipelineImpl_HandleInputRequest_NilRequest(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	node := NewDGANode("test", StatusUnknown)
	// event.Request 为 nil 不应 panic
	pipeline.handleInputRequest(node, &executor.InputRequestEvent{})
}

func TestPipelineImpl_HandleInputRequest_NilRuntimeStatus(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	node := NewDGANode("test", StatusUnknown)
	// 不设置 RuntimeStatus
	pipeline.handleInputRequest(node, &executor.InputRequestEvent{
		Request: &executor.InputRequest{},
	})
	// 不应 panic
}

func TestPipelineImpl_HandleInputRequest_Valid(t *testing.T) {
	pipeline := NewPipeline(context.Background()).(*PipelineImpl)
	node := NewDGANode("test", StatusUnknown)
	node.SetRuntimeStatus(&NodeRuntimeStatus{
		Id:     "rt-1",
		Status: StatusRunning,
	})

	pipeline.handleInputRequest(node, &executor.InputRequestEvent{
		StepName: "step1",
		Request: &executor.InputRequest{
			Prompt: "Enter value:",
			Type:   "text",
		},
	})

	rt := node.GetRuntimeStatus()
	if rt.Status != StatusPaused {
		t.Errorf("Status = %q, want %q", rt.Status, StatusPaused)
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
			node:     NewDGANode("n1", StatusUnknown),
			stepName: "step1",
			want:     "unknown",
		},
		{
			name:     "step found",
			setupNode: func() Node {
				n := NewDGANode("n2", StatusRunning)
				n.SetRuntimeStatus(&NodeRuntimeStatus{
					Steps: []StepRuntimeStatus{
						{Name: "step1", Status: StatusSuccess},
						{Name: "step2", Status: StatusFailed},
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
				n := NewDGANode("n3", StatusRunning)
				n.SetRuntimeStatus(&NodeRuntimeStatus{
					Steps: []StepRuntimeStatus{
						{Name: "step1", Status: StatusSuccess},
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
	node := NewDGANodeWithConfig("Node1", StatusUnknown, "local", "", []Step{
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
