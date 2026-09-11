package dag

import (
	"testing"

	"github.com/LerkoX/flowx/core"
)

func TestDGAEvaluationContext_All_Basic(t *testing.T) {
	ctx := NewEvaluationContext()
	ctx.(*DGAEvaluationContext).data = map[string]any{
		"customKey": "customValue",
	}

	all := ctx.All()

	if all["customKey"] != "customValue" {
		t.Errorf("All()[\"customKey\"] = %v, want customValue", all["customKey"])
	}
	if all["iteration"] != 0 {
		t.Errorf("All()[\"iteration\"] = %v, want 0", all["iteration"])
	}
}

func TestDGAEvaluationContext_All_WithWorkflow(t *testing.T) {
	graph := NewDGAGraph()
	workflow := NewWorkflow(nil)
	workflow.(*WorkflowImpl).SetGraph(graph)
	workflow.(*WorkflowImpl).setId("workflow-123")

	ctx := NewEvaluationContext().WithWorkflow(workflow)

	all := ctx.All()

	if all["workflowId"] != "workflow-123" {
		t.Errorf("All()[\"workflowId\"] = %v, want workflow-123", all["workflowId"])
	}
	// Status is empty string by default (PENDING is only set during run)
	if _, exists := all["workflowStatus"]; exists {
		t.Logf("workflowStatus present: %v", all["workflowStatus"])
	}
}

func TestDGAEvaluationContext_All_WithWorkflowAndParam(t *testing.T) {
	graph := NewDGAGraph()
	workflow := NewWorkflow(nil)
	workflow.(*WorkflowImpl).SetGraph(graph)
	workflow.(*WorkflowImpl).setId("workflow-param-test")

	// Set param using FieldItem
	param := map[string]interface{}{
		"env":      "production",
		"version":  "v1.0.0",
		"replicas": 3,
		"enabled":   true,
	}
	workflow.(*WorkflowImpl).SetParam(param)

	ctx := NewEvaluationContext().WithWorkflow(workflow)

	all := ctx.All()

	// Check direct param access
	if all["env"] != "production" {
		t.Errorf("All()[\"env\"] = %v, want production", all["env"])
	}
	if all["version"] != "v1.0.0" {
		t.Errorf("All()[\"version\"] = %v, want v1.0.0", all["version"])
	}
	if all["replicas"] != 3 {
		t.Errorf("All()[\"replicas\"] = %v, want 3", all["replicas"])
	}
	// Boolean values in Param should be converted to strings in context
	if all["enabled"] != "true" {
		t.Errorf("All()[\"enabled\"] = %v, want \"true\"", all["enabled"])
	}

	// Check Param.xxx access
	paramMap, ok := all["Param"].(map[string]any)
	if !ok {
		t.Fatalf("All()[\"Param\"] is not a map")
	}
	if paramMap["env"] != "production" {
		t.Errorf("All()[\"Param\"][\"env\"] = %v, want production", paramMap["env"])
	}
	// Boolean in Param map should be converted to string
	if paramMap["enabled"] != "true" {
		t.Errorf("All()[\"Param\"][\"enabled\"] = %v, want \"true\"", paramMap["enabled"])
	}
}

func TestDGAEvaluationContext_All_WithWorkflowAndMetadata(t *testing.T) {
	graph := NewDGAGraph()
	workflow := NewWorkflow(nil)
	workflow.(*WorkflowImpl).SetGraph(graph)
	workflow.(*WorkflowImpl).setId("workflow-meta-test")

	// Set param
	param := map[string]interface{}{
		"projectName": "myapp",
	}
	workflow.(*WorkflowImpl).SetParam(param)

	// Simulate setting metadata (using internal metadata map)
	// Note: In real usage, metadata is set through workflow operations
	workflowImpl := workflow.(*WorkflowImpl)
	workflowImpl.mu.Lock()
	workflowImpl.metadata = Metadata{
		"Build.buildId":    core.FieldItem{Value: "12345", SrcNode: "Build"},
		"Build.status":      core.FieldItem{Value: "success", SrcNode: "Build"},
		"Deploy.namespace":  core.FieldItem{Value: "prod", SrcNode: "Deploy"},
		"standaloneKey":    core.FieldItem{Value: "standalone", SrcNode: ""},
	}
	workflowImpl.mu.Unlock()

	ctx := NewEvaluationContext().WithWorkflow(workflow)

	all := ctx.All()

	// Check nested structure for Build
	build, ok := all["Build"].(map[string]any)
	if !ok {
		t.Fatalf("All()[\"Build\"] is not a map")
	}
	if build["buildId"] != "12345" {
		t.Errorf("All()[\"Build\"][\"buildId\"] = %v, want 12345", build["buildId"])
	}
	if build["status"] != "success" {
		t.Errorf("All()[\"Build\"][\"status\"] = %v, want success", build["status"])
	}

	// Check nested structure for Deploy
	deploy, ok := all["Deploy"].(map[string]any)
	if !ok {
		t.Fatalf("All()[\"Deploy\"] is not a map")
	}
	if deploy["namespace"] != "prod" {
		t.Errorf("All()[\"Deploy\"][\"namespace\"] = %v, want prod", deploy["namespace"])
	}

	// Check standalone key
	if all["standaloneKey"] != "standalone" {
		t.Errorf("All()[\"standaloneKey\"] = %v, want standalone", all["standaloneKey"])
	}
}

func TestDGAEvaluationContext_All_MultipleSources(t *testing.T) {
	graph := NewDGAGraph()
	node := NewDGANode("test-node", "RUNNING")
	graph.AddVertex(node)
	workflow := NewWorkflow(nil)
	workflow.(*WorkflowImpl).SetGraph(graph)
	workflow.(*WorkflowImpl).setId("workflow-multi-test")

	// Set param
	param := map[string]interface{}{
		"env": "staging",
	}
	workflow.(*WorkflowImpl).SetParam(param)

	// Set metadata
	workflowImpl := workflow.(*WorkflowImpl)
	workflowImpl.mu.Lock()
	workflowImpl.metadata = Metadata{
		"Node1.value": core.FieldItem{Value: "test-value", SrcNode: "Node1"},
	}
	workflowImpl.mu.Unlock()

	ctx := NewEvaluationContext().WithNode(node).WithWorkflow(workflow).WithIteration(5)

	all := ctx.All()

	// Verify all sources are present
	if all["workflowId"] != "workflow-multi-test" {
		t.Errorf("workflowId = %v, want workflow-multi-test", all["workflowId"])
	}
	if all["nodeId"] != "test-node" {
		t.Errorf("nodeId = %v, want test-node", all["nodeId"])
	}
	if all["nodeStatus"] != "RUNNING" {
		t.Errorf("nodeStatus = %v, want RUNNING", all["nodeStatus"])
	}
	if all["iteration"] != 5 {
		t.Errorf("iteration = %v, want 5", all["iteration"])
	}
	if all["env"] != "staging" {
		t.Errorf("env = %v, want staging", all["env"])
	}
}

func TestDGAEvaluationContext_WithNode_Immutability(t *testing.T) {
	original := NewEvaluationContext()
	original.(*DGAEvaluationContext).data = map[string]any{"key": "original"}

	modified := original.WithNode(NewDGANode("node1", "SUCCESS"))

	// Original should not have node data
	allOrig := original.All()
	if _, exists := allOrig["nodeId"]; exists {
		t.Error("Original context should not have nodeId")
	}

	// Modified should have node data
	allMod := modified.All()
	if allMod["nodeId"] != "node1" {
		t.Errorf("Modified context nodeId = %v, want node1", allMod["nodeId"])
	}
}

func TestDGAEvaluationContext_WithWorkflow_Immutability(t *testing.T) {
	original := NewEvaluationContext()
	original.(*DGAEvaluationContext).data = map[string]any{"custom": "value"}

	graph := NewDGAGraph()
	workflow := NewWorkflow(nil)
	workflow.(*WorkflowImpl).SetGraph(graph)
	workflow.(*WorkflowImpl).setId("p1")

	modified := original.WithWorkflow(workflow)

	// Original should not have workflow data
	allOrig := original.All()
	if _, exists := allOrig["workflowId"]; exists {
		t.Error("Original context should not have workflowId")
	}

	// Modified should have workflow data
	allMod := modified.All()
	if allMod["workflowId"] != "p1" {
		t.Errorf("Modified context workflowId = %v, want p1", allMod["workflowId"])
	}

	// Original data should be preserved
	if allMod["custom"] != "value" {
		t.Errorf("Modified context custom = %v, want value", allMod["custom"])
	}
}

func TestDGAEvaluationContext_WithParams(t *testing.T) {
	original := NewEvaluationContext()
	original.(*DGAEvaluationContext).data = map[string]any{"existing": "data"}

	params := map[string]any{
		"newParam": "newValue",
		"number":    42,
	}

	modified := original.WithParams(params)

	// Original should not have new params
	allOrig := original.All()
	if _, exists := allOrig["newParam"]; exists {
		t.Error("Original context should not have newParam")
	}

	// Modified should have all params
	allMod := modified.All()
	if allMod["newParam"] != "newValue" {
		t.Errorf("Modified context newParam = %v, want newValue", allMod["newParam"])
	}
	if allMod["number"] != 42 {
		t.Errorf("Modified context number = %v, want 42", allMod["number"])
	}
	if allMod["existing"] != "data" {
		t.Errorf("Modified context existing = %v, want data", allMod["existing"])
	}
}

func TestDGAEvaluationContext_WithIteration_Chained(t *testing.T) {
	ctx := NewEvaluationContext().
		WithIteration(1).
		WithIteration(2).
		WithIteration(3)

	if ctx.Iteration() != 3 {
		t.Errorf("Iteration() = %d, want 3", ctx.Iteration())
	}

	// All() should return the final iteration value
	all := ctx.All()
	if all["iteration"] != 3 {
		t.Errorf("All()[\"iteration\"] = %v, want 3", all["iteration"])
	}
}

func TestDGAEvaluationContext_NilWorkflow(t *testing.T) {
	ctx := NewEvaluationContext()
	// Don't set workflow, leave it nil

	all := ctx.All()

	// Should not panic and should not have workflow-related keys
	if _, exists := all["workflowId"]; exists {
		t.Error("Nil workflow should not add workflowId to All()")
	}
	if _, exists := all["workflowStatus"]; exists {
		t.Error("Nil workflow should not add workflowStatus to All()")
	}
}

func TestDGAEvaluationContext_NilNode(t *testing.T) {
	ctx := NewEvaluationContext()
	// Don't set node, leave it nil

	all := ctx.All()

	// Should not panic and should not have node-related keys
	if _, exists := all["nodeId"]; exists {
		t.Error("Nil node should not add nodeId to All()")
	}
	if _, exists := all["nodeStatus"]; exists {
		t.Error("Nil node should not add nodeStatus to All()")
	}
}

// Helper function to set workflow ID for testing
func (p *WorkflowImpl) setId(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.id = id
}

