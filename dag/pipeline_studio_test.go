package dag

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/LerkoX/flowx/core"
)

// TestCurrentNode_InitialState 测试初始状态 CurrentNode 返回 nil
func TestCurrentNode_InitialState(t *testing.T) {
	pipeline := NewPipeline(context.Background())
	impl := pipeline.(*PipelineImpl)

	node := impl.CurrentNode()
	if node != nil {
		t.Errorf("Expected CurrentNode to be nil initially, got %v", node)
	}
}

// TestCurrentNode_SetAndGet 测试设置和获取当前节点
func TestCurrentNode_SetAndGet(t *testing.T) {
	pipeline := NewPipeline(context.Background())
	impl := pipeline.(*PipelineImpl)

	// 创建一个测试节点
	testNode := NewDGANode("test-node", core.StatusRunning)

	// 手动设置当前节点（模拟执行过程中）
	impl.mu.Lock()
	impl.currentNode = testNode
	impl.mu.Unlock()

	// 验证 CurrentNode 返回正确的节点
	node := impl.CurrentNode()
	if node == nil {
		t.Fatal("Expected CurrentNode to not be nil")
	}
	if node.Id() != "test-node" {
		t.Errorf("Expected node ID 'test-node', got %s", node.Id())
	}

	// 清理后验证返回 nil
	impl.mu.Lock()
	impl.currentNode = nil
	impl.mu.Unlock()

	node = impl.CurrentNode()
	if node != nil {
		t.Errorf("Expected CurrentNode to be nil after cleanup, got %v", node)
	}
}

// TestCurrentNode_ConcurrentAccess 测试 CurrentNode 的并发安全性
func TestCurrentNode_ConcurrentAccess(t *testing.T) {
	pipeline := NewPipeline(context.Background())
	impl := pipeline.(*PipelineImpl)

	testNode := NewDGANode("concurrent-node", core.StatusRunning)

	// 并发写入和读取
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			impl.mu.Lock()
			impl.currentNode = testNode
			impl.mu.Unlock()
			time.Sleep(time.Microsecond)
			impl.mu.Lock()
			impl.currentNode = nil
			impl.mu.Unlock()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = impl.CurrentNode()
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	// 等待两个 goroutine 完成
	<-done
	<-done
}

// TestPipelineNodeFailed_Event 测试节点失败时触发 PipelineNodeFailed 事件
func TestPipelineNodeFailed_Event(t *testing.T) {
	// 创建一个事件监听器来捕获事件
	eventCollector := &testEventCollector{}
	listener := &testEventListener{
		collector: eventCollector,
	}

	pipeline := NewPipeline(context.Background())
	impl := pipeline.(*PipelineImpl)
	impl.Listening(listener)

	// 验证 PipelineNodeFailed 事件常量已定义
	if PipelineNodeFailed != core.EventPipelineNodeFailed {
		t.Errorf("PipelineNodeFailed event mismatch: expected %s, got %s",
			core.EventPipelineNodeFailed, PipelineNodeFailed)
	}

	// 手动触发 PipelineNodeFailed 事件
	impl.NotifyEvent(PipelineNodeFailed)

	// 验证监听器收到了事件
	time.Sleep(50 * time.Millisecond) // 给事件处理一点时间

	found := false
	for _, event := range eventCollector.events {
		if event == PipelineNodeFailed {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected PipelineNodeFailed event to be received by listener")
	}
}

// TestCurrentNode_DuringExecution 测试节点执行过程中 CurrentNode 的设置和清理
func TestCurrentNode_DuringExecution(t *testing.T) {
	ctx := context.Background()
	pipeline := NewPipeline(ctx)
	impl := pipeline.(*PipelineImpl)

	// 创建一个测试节点
	testNode := NewDGANodeWithConfig("execution-node", core.StatusRunning, "test-executor", "", nil, nil)

	// 验证执行前 CurrentNode 为 nil
	if impl.CurrentNode() != nil {
		t.Error("Expected CurrentNode to be nil before execution")
	}

	// 模拟 executeNodeWithLifecycle 中的设置
	impl.mu.Lock()
	impl.currentNode = testNode
	impl.mu.Unlock()

	// 验证执行中 CurrentNode 不为 nil
	node := impl.CurrentNode()
	if node == nil {
		t.Fatal("Expected CurrentNode to not be nil during execution")
	}
	if node.Id() != "execution-node" {
		t.Errorf("Expected node ID 'execution-node', got %s", node.Id())
	}

	// 模拟执行完成后的清理
	impl.mu.Lock()
	impl.currentNode = nil
	impl.mu.Unlock()

	// 验证执行后 CurrentNode 为 nil
	if impl.CurrentNode() != nil {
		t.Error("Expected CurrentNode to be nil after execution")
	}
}

// testEventCollector 用于收集测试事件
type testEventCollector struct {
	events []Event
	mu     sync.Mutex
}

// testEventListener 测试用的事件监听器
type testEventListener struct {
	collector *testEventCollector
}

func (l *testEventListener) Handle(p Pipeline, event Event) {
	if l.collector != nil {
		l.collector.mu.Lock()
		l.collector.events = append(l.collector.events, event)
		l.collector.mu.Unlock()
	}
}

func (l *testEventListener) Events() []Event {
	return []Event{
		PipelineNodeStart,
		PipelineNodeFinish,
		PipelineNodeFailed,
	}
}

// 确保 testEventListener 实现了 Listener 接口
var _ Listener = (*testEventListener)(nil)

// TestPipelineNodeFailed_ConstantValue 测试 PipelineNodeFailed 常量值
func TestPipelineNodeFailed_ConstantValue(t *testing.T) {
	expected := Event("pipeline-node-failed")
	if PipelineNodeFailed != expected {
		t.Errorf("PipelineNodeFailed constant mismatch: expected %v, got %v", expected, PipelineNodeFailed)
	}
}

// TestCurrentNode_ThreadSafety 测试 CurrentNode 在多个 goroutine 中的线程安全性
func TestCurrentNode_ThreadSafety(t *testing.T) {
	pipeline := NewPipeline(context.Background())
	impl := pipeline.(*PipelineImpl)

	node1 := NewDGANode("node-1", core.StatusRunning)
	node2 := NewDGANode("node-2", core.StatusRunning)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			impl.mu.Lock()
			impl.currentNode = node1
			impl.mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			impl.mu.Lock()
			impl.currentNode = node2
			impl.mu.Unlock()
			_ = impl.CurrentNode()
		}()
	}

	wg.Wait()
}
