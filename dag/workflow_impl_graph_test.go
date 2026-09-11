package dag

import (
	"testing"

	"github.com/LerkoX/flowx/core"
)

// TestDAGGraph_Edges 测试获取所有边
func TestDAGGraph_Edges(t *testing.T) {
	graph := NewDGAGraph()
	node1 := NewDGANode("node1", "RUNNING")
	node2 := NewDGANode("node2", "RUNNING")
	graph.AddVertex(node1)
	graph.AddVertex(node2)

	// 添加边
	edge := NewDGAEdge(node1, node2)
	graph.AddEdge(edge)

	edges := graph.Edges()
	if len(edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(edges))
	}
}

// TestDAGGraph_HasCycle 测试无环检测
func TestDAGGraph_HasCycle(t *testing.T) {
	graph := NewDGAGraph()
	node1 := NewDGANode("node1", "RUNNING")
	node2 := NewDGANode("node2", "RUNNING")
	graph.AddVertex(node1)
	graph.AddVertex(node2)

	// 添加边
	edge := NewDGAEdge(node1, node2)
	graph.AddEdge(edge)

	// 没有环
	if graph.HasCycle() {
		t.Error("Graph without cycle should return false")
	}
}

// TestDAGGraph_GetEdge 测试获取特定边
func TestDAGGraph_GetEdge(t *testing.T) {
	graph := NewDGAGraph()
	node1 := NewDGANode("node1", "RUNNING")
	node2 := NewDGANode("node2", "RUNNING")
	graph.AddVertex(node1)
	graph.AddVertex(node2)

	// 添加边
	edge := NewDGAEdge(node1, node2)
	graph.AddEdge(edge)

	// 获取存在的边
	gotEdge, exists := graph.GetEdge("node1", "node2")
	if !exists {
		t.Error("Expected to find edge")
	}
	if gotEdge == nil {
		t.Error("Edge should not be nil")
	}

	// 获取不存在的边
	_, exists = graph.GetEdge("node2", "node1")
	if exists {
		t.Error("Should not find edge from node2 to node1")
	}
}

// TestDAGGraph_IncomingEdges 测试获取入边
func TestDAGGraph_IncomingEdges(t *testing.T) {
	graph := NewDGAGraph()
	node1 := NewDGANode("node1", "RUNNING")
	node2 := NewDGANode("node2", "RUNNING")
	graph.AddVertex(node1)
	graph.AddVertex(node2)

	// node1 -> node2
	edge := NewDGAEdge(node1, node2)
	graph.AddEdge(edge)

	// node2 的入边
	incoming := graph.IncomingEdges("node2")
	if len(incoming) != 1 {
		t.Errorf("Expected 1 incoming edge, got %d", len(incoming))
	}

	// node1 的入边
	incoming = graph.IncomingEdges("node1")
	if len(incoming) != 0 {
		t.Errorf("Expected 0 incoming edges for node1, got %d", len(incoming))
	}
}

// TestDAGGraph_OutgoingEdges 测试获取出边
func TestDAGGraph_OutgoingEdges(t *testing.T) {
	graph := NewDGAGraph()
	node1 := NewDGANode("node1", "RUNNING")
	node2 := NewDGANode("node2", "RUNNING")
	graph.AddVertex(node1)
	graph.AddVertex(node2)

	// node1 -> node2
	edge := NewDGAEdge(node1, node2)
	graph.AddEdge(edge)

	// node1 的出边
	outgoing := graph.OutgoingEdges("node1")
	if len(outgoing) != 1 {
		t.Errorf("Expected 1 outgoing edge, got %d", len(outgoing))
	}

	// node2 的出边
	outgoing = graph.OutgoingEdges("node2")
	if len(outgoing) != 0 {
		t.Errorf("Expected 0 outgoing edges for node2, got %d", len(outgoing))
	}
}

// TestDAGGraph_RemoveVertex 测试删除节点
func TestDAGGraph_RemoveVertex(t *testing.T) {
	graph := NewDGAGraph()
	node1 := NewDGANode("node1", "RUNNING")
	node2 := NewDGANode("node2", "RUNNING")
	graph.AddVertex(node1)
	graph.AddVertex(node2)

	// 添加边
	edge := NewDGAEdge(node1, node2)
	graph.AddEdge(edge)

	// 删除 node1
	err := graph.RemoveVertex("node1")
	if err != nil {
		t.Errorf("RemoveVertex failed: %v", err)
	}

	// 验证 node1 已删除
	_, exists := graph.GetNode("node1")
	if exists {
		t.Error("Node1 should be deleted")
	}

	// 验证 node2 还在
	_, exists = graph.GetNode("node2")
	if !exists {
		t.Error("Node2 should still exist")
	}
}

// TestDAGGraph_RemoveEdge 测试删除边
func TestDAGGraph_RemoveEdge(t *testing.T) {
	graph := NewDGAGraph()
	node1 := NewDGANode("node1", "RUNNING")
	node2 := NewDGANode("node2", "RUNNING")
	graph.AddVertex(node1)
	graph.AddVertex(node2)

	// 添加边
	edge := NewDGAEdge(node1, node2)
	graph.AddEdge(edge)

	// 删除边
	err := graph.RemoveEdge("node1", "node2")
	if err != nil {
		t.Errorf("RemoveEdge failed: %v", err)
	}

	// 验证边已删除
	_, exists := graph.GetEdge("node1", "node2")
	if exists {
		t.Error("Edge should be deleted")
	}
}

// TestDAGGraph_SetMetadata 测试设置元数据
func TestDAGGraph_SetMetadata(t *testing.T) {
	workflow := NewWorkflow(nil)
	impl := workflow.(*WorkflowImpl)

	impl.SetMetadata(nil)

	// 验证不 panic
	metadata := impl.Metadata()
	if metadata == nil {
		t.Log("Metadata can be nil")
	}
}

// TestDGANode_Get 测试获取节点属性
func TestDGANode_Get(t *testing.T) {
	node := NewDGANode("test-node", "RUNNING")
	node.Set("key1", "value1")

	val := node.Get("key1")
	if val != "value1" {
		t.Errorf("Expected 'value1', got '%s'", val)
	}

	// 获取不存在的键
	val = node.Get("nonexistent")
	if val != "" {
		t.Errorf("Expected empty string for nonexistent key, got '%s'", val)
	}
}

// TestDGANode_Set 测试设置节点属性
func TestDGANode_Set(t *testing.T) {
	node := NewDGANode("test-node", "RUNNING")

	node.Set("key1", "value1")
	node.Set("key2", 42)

	if node.Get("key1") != "value1" {
		t.Error("key1 should be 'value1'")
	}
}

// TestDGANode_GetStepRuntimeStatus 测试获取步骤运行时状态
func TestDGANode_GetStepRuntimeStatus(t *testing.T) {
	node := NewDGANode("test-node", "RUNNING")

	// 设置步骤状态
	node.SetStepRuntimeStatus(&core.StepRuntimeStatus{
		Name:   "step1",
		Status: core.StatusRunning,
	})

	// 获取步骤状态
	step := node.GetStepRuntimeStatus("step1")
	if step == nil {
		t.Fatal("Expected to find step1")
	}
	if step.Status != core.StatusRunning {
		t.Errorf("Expected StatusRunning, got %s", step.Status)
	}

	// 获取不存在的步骤
	step = node.GetStepRuntimeStatus("nonexistent")
	if step != nil {
		t.Error("Nonexistent step should return nil")
	}
}

// TestDGANode_WorkflowId 测试获取节点所属流水线ID
func TestDGANode_WorkflowId(t *testing.T) {
	node := NewDGANode("test-node", "RUNNING")

	pid := node.WorkflowId()
	if pid != "" {
		t.Errorf("Expected empty string, got '%s'", pid)
	}
}

// Note: handleCancellation is private and takes nodeContext interface,
// which cannot be easily tested from external tests.
