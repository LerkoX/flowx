package flowx

import (
	"context"
	"testing"

	"github.com/LerkoX/flowx/core"
	"github.com/LerkoX/flowx/dag"
)

// collectEdges 提取边 ID（格式 "source->target"）
func collectEdges(g *dag.DGAGraph) []string {
	var out []string
	for _, e := range g.Edges() {
		out = append(out, e.ID())
	}
	return out
}

func hasEdge(edges []string, from, to string) bool {
	want := from + "->" + to
	for _, id := range edges {
		if id == want {
			return true
		}
	}
	return false
}

// TestParseGraphEdges_FlowchartBasic 测试基本 flowchart 解析（graph TD 头）
func TestParseGraphEdges_FlowchartBasic(t *testing.T) {
	ctx := context.Background()
	config := &core.WorkflowConfig{
		Nodes: map[string]core.NodeConfig{
			"Merge":  {},
			"Build":  {},
			"Deploy": {},
		},
		Graph: `flowchart TD
    [*] --> Merge
    Merge --> Build
    Build --> Deploy
    Deploy --> [*]`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	nodes := graph.Nodes()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}

	dga, ok := graph.(*dag.DGAGraph)
	if !ok {
		t.Fatalf("Expected *dag.DGAGraph, got %T", graph)
	}
	edges := collectEdges(dga)
	for _, want := range [][2]string{{"Merge", "Build"}, {"Build", "Deploy"}} {
		if !hasEdge(edges, want[0], want[1]) {
			t.Errorf("Expected edge %s -> %s not found, edges: %v", want[0], want[1], edges)
		}
	}
	if len(edges) != 2 {
		t.Errorf("Expected 2 edges, got %d: %v", len(edges), edges)
	}
}

// TestParseGraphEdges_FlowchartSubgraph 测试 subgraph 分组内边被递归拍平
func TestParseGraphEdges_FlowchartSubgraph(t *testing.T) {
	ctx := context.Background()
	config := &core.WorkflowConfig{
		Nodes: map[string]core.NodeConfig{
			"Load":    {},
			"Grow":    {},
			"Feather": {},
			"Sample":  {},
			"Save":    {},
		},
		Graph: `flowchart LR
    [*] --> Load
    subgraph Mask[蒙版处理]
        Grow --> Feather
    end
    Load --> Grow
    Feather --> Sample
    Load --> Sample
    Sample --> Save
    Save --> [*]`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	dga := graph.(*dag.DGAGraph)
	edges := collectEdges(dga)
	wants := [][2]string{
		{"Grow", "Feather"},   // subgraph 内部边
		{"Load", "Grow"},      // 跨组入边
		{"Feather", "Sample"}, // 跨组出边
		{"Load", "Sample"},
		{"Sample", "Save"},
	}
	for _, want := range wants {
		if !hasEdge(edges, want[0], want[1]) {
			t.Errorf("Expected edge %s -> %s not found, edges: %v", want[0], want[1], edges)
		}
	}
	if len(edges) != len(wants) {
		t.Errorf("Expected %d edges, got %d: %v", len(wants), len(edges), edges)
	}
}

// TestParseGraphEdges_FlowchartNestedSubgraph 测试嵌套 subgraph
func TestParseGraphEdges_FlowchartNestedSubgraph(t *testing.T) {
	ctx := context.Background()
	config := &core.WorkflowConfig{
		Nodes: map[string]core.NodeConfig{
			"A": {}, "B": {}, "C": {}, "D": {},
		},
		Graph: `flowchart TD
    subgraph Outer[外层]
        subgraph Inner[内层]
            A --> B
        end
        B --> C
    end
    C --> D`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	dga := graph.(*dag.DGAGraph)
	edges := collectEdges(dga)
	wants := [][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}}
	for _, want := range wants {
		if !hasEdge(edges, want[0], want[1]) {
			t.Errorf("Expected edge %s -> %s not found, edges: %v", want[0], want[1], edges)
		}
	}
}

// TestParseGraphEdges_FlowchartConditionalEdge 测试 flowchart 条件边（|label| 语法）
func TestParseGraphEdges_FlowchartConditionalEdge(t *testing.T) {
	ctx := context.Background()
	config := &core.WorkflowConfig{
		Nodes: map[string]core.NodeConfig{
			"Check": {},
			"Retry": {},
			"Done":  {},
		},
		Graph: `flowchart TD
    Check -->|{{ iteration < 3 }}| Retry
    Check --> Done`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	dga := graph.(*dag.DGAGraph)
	var condExpr string
	plainCount := 0
	for _, e := range dga.Edges() {
		if e.ID() == "Check->Retry" {
			condExpr = e.Expression()
		}
		if e.ID() == "Check->Done" && e.Expression() == "" {
			plainCount++
		}
	}
	if condExpr != "{{ iteration < 3 }}" {
		t.Errorf("Expected conditional expression '{{ iteration < 3 }}', got %q", condExpr)
	}
	if plainCount != 1 {
		t.Errorf("Expected 1 plain edge Check->Done, got %d", plainCount)
	}
}

// TestParseGraphEdges_FlowchartEntryExit 测试 flowchart 的 [*] 起止标记
func TestParseGraphEdges_FlowchartEntryExit(t *testing.T) {
	ctx := context.Background()
	config := &core.WorkflowConfig{
		Nodes: map[string]core.NodeConfig{
			"A": {}, "B": {},
		},
		Graph: `flowchart TD
    [*] --> A
    A --> B
    B --> [*]`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	dga := graph.(*dag.DGAGraph)
	entry := dga.EntryNodes()
	if len(entry) != 1 || entry[0] != "A" {
		t.Errorf("Expected entry node [A], got %v", entry)
	}
	exit := dga.ExitNodes()
	if len(exit) != 1 || exit[0] != "B" {
		t.Errorf("Expected exit node [B], got %v", exit)
	}
}

// TestParseGraphEdges_FlowchartGraphKeyword 测试 graph 关键字（flowchart 的旧写法）
func TestParseGraphEdges_FlowchartGraphKeyword(t *testing.T) {
	ctx := context.Background()
	config := &core.WorkflowConfig{
		Nodes: map[string]core.NodeConfig{
			"A": {}, "B": {},
		},
		Graph: `graph LR
    A --> B`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	dga := graph.(*dag.DGAGraph)
	edges := collectEdges(dga)
	if !hasEdge(edges, "A", "B") {
		t.Errorf("Expected edge A -> B not found, edges: %v", edges)
	}
}

// TestParseGraphEdges_FlowchartInlineNodeDef 测试行内节点定义（A[标签]）不影响边解析
func TestParseGraphEdges_FlowchartInlineNodeDef(t *testing.T) {
	ctx := context.Background()
	config := &core.WorkflowConfig{
		Nodes: map[string]core.NodeConfig{
			"A": {}, "B": {},
		},
		Graph: `flowchart TD
    A[加载模型] --> B[采样]`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	dga := graph.(*dag.DGAGraph)
	edges := collectEdges(dga)
	if !hasEdge(edges, "A", "B") {
		t.Errorf("Expected edge A -> B not found, edges: %v", edges)
	}
}

// TestParseGraphEdges_StateDiagramUnaffected 回归：stateDiagram-v2 路径不受影响
func TestParseGraphEdges_StateDiagramUnaffected(t *testing.T) {
	ctx := context.Background()
	config := &core.WorkflowConfig{
		Nodes: map[string]core.NodeConfig{
			"A": {}, "B": {}, "C": {},
		},
		Graph: `stateDiagram-v2
    [*] --> A
    A --> B: {{ iteration < 3 }}
    B --> C
    C --> [*]`,
	}

	runtime := NewRuntime(ctx).(*RuntimeImpl)
	graph := runtime.buildGraph(config)

	dga := graph.(*dag.DGAGraph)
	edges := collectEdges(dga)
	if !hasEdge(edges, "A", "B") || !hasEdge(edges, "B", "C") {
		t.Errorf("stateDiagram edges missing: %v", edges)
	}
	for _, e := range dga.Edges() {
		if e.ID() == "A->B" && e.Expression() != "{{ iteration < 3 }}" {
			t.Errorf("Expected conditional expression, got %q", e.Expression())
		}
	}
}
