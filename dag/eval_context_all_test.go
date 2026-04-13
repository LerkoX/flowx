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

func TestDGAEvaluationContext_All_WithPipeline(t *testing.T) {
	graph := NewDGAGraph()
	pipeline := NewPipeline(nil)
	pipeline.(*PipelineImpl).SetGraph(graph)
	pipeline.(*PipelineImpl).setId("pipeline-123")

	ctx := NewEvaluationContext().WithPipeline(pipeline)

	all := ctx.All()

	if all["pipelineId"] != "pipeline-123" {
		t.Errorf("All()[\"pipelineId\"] = %v, want pipeline-123", all["pipelineId"])
	}
	// Status is empty string by default (PENDING is only set during run)
	if _, exists := all["pipelineStatus"]; exists {
		t.Logf("pipelineStatus present: %v", all["pipelineStatus"])
	}
}

func TestDGAEvaluationContext_All_WithPipelineAndParam(t *testing.T) {
	graph := NewDGAGraph()
	pipeline := NewPipeline(nil)
	pipeline.(*PipelineImpl).SetGraph(graph)
	pipeline.(*PipelineImpl).setId("pipeline-param-test")

	// Set param using FieldItem
	param := map[string]interface{}{
		"env":      "production",
		"version":  "v1.0.0",
		"replicas": 3,
		"enabled":   true,
	}
	pipeline.(*PipelineImpl).SetParam(param)

	ctx := NewEvaluationContext().WithPipeline(pipeline)

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

func TestDGAEvaluationContext_All_WithPipelineAndMetadata(t *testing.T) {
	graph := NewDGAGraph()
	pipeline := NewPipeline(nil)
	pipeline.(*PipelineImpl).SetGraph(graph)
	pipeline.(*PipelineImpl).setId("pipeline-meta-test")

	// Set param
	param := map[string]interface{}{
		"projectName": "myapp",
	}
	pipeline.(*PipelineImpl).SetParam(param)

	// Simulate setting metadata (using internal metadata map)
	// Note: In real usage, metadata is set through pipeline operations
	pipelineImpl := pipeline.(*PipelineImpl)
	pipelineImpl.mu.Lock()
	pipelineImpl.metadata = Metadata{
		"Build.buildId":    core.FieldItem{Value: "12345", SrcNode: "Build"},
		"Build.status":      core.FieldItem{Value: "success", SrcNode: "Build"},
		"Deploy.namespace":  core.FieldItem{Value: "prod", SrcNode: "Deploy"},
		"standaloneKey":    core.FieldItem{Value: "standalone", SrcNode: ""},
	}
	pipelineImpl.mu.Unlock()

	ctx := NewEvaluationContext().WithPipeline(pipeline)

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
	pipeline := NewPipeline(nil)
	pipeline.(*PipelineImpl).SetGraph(graph)
	pipeline.(*PipelineImpl).setId("pipeline-multi-test")

	// Set param
	param := map[string]interface{}{
		"env": "staging",
	}
	pipeline.(*PipelineImpl).SetParam(param)

	// Set metadata
	pipelineImpl := pipeline.(*PipelineImpl)
	pipelineImpl.mu.Lock()
	pipelineImpl.metadata = Metadata{
		"Node1.value": core.FieldItem{Value: "test-value", SrcNode: "Node1"},
	}
	pipelineImpl.mu.Unlock()

	ctx := NewEvaluationContext().WithNode(node).WithPipeline(pipeline).WithIteration(5)

	all := ctx.All()

	// Verify all sources are present
	if all["pipelineId"] != "pipeline-multi-test" {
		t.Errorf("pipelineId = %v, want pipeline-multi-test", all["pipelineId"])
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

func TestDGAEvaluationContext_WithPipeline_Immutability(t *testing.T) {
	original := NewEvaluationContext()
	original.(*DGAEvaluationContext).data = map[string]any{"custom": "value"}

	graph := NewDGAGraph()
	pipeline := NewPipeline(nil)
	pipeline.(*PipelineImpl).SetGraph(graph)
	pipeline.(*PipelineImpl).setId("p1")

	modified := original.WithPipeline(pipeline)

	// Original should not have pipeline data
	allOrig := original.All()
	if _, exists := allOrig["pipelineId"]; exists {
		t.Error("Original context should not have pipelineId")
	}

	// Modified should have pipeline data
	allMod := modified.All()
	if allMod["pipelineId"] != "p1" {
		t.Errorf("Modified context pipelineId = %v, want p1", allMod["pipelineId"])
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

func TestDGAEvaluationContext_NilPipeline(t *testing.T) {
	ctx := NewEvaluationContext()
	// Don't set pipeline, leave it nil

	all := ctx.All()

	// Should not panic and should not have pipeline-related keys
	if _, exists := all["pipelineId"]; exists {
		t.Error("Nil pipeline should not add pipelineId to All()")
	}
	if _, exists := all["pipelineStatus"]; exists {
		t.Error("Nil pipeline should not add pipelineStatus to All()")
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

// Helper function to set pipeline ID for testing
func (p *PipelineImpl) setId(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.id = id
}

