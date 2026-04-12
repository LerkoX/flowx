package dag

import "testing"

func TestDGAEvaluationContext_Iteration_Default(t *testing.T) {
	ctx := NewEvaluationContext()
	if ctx.Iteration() != 0 {
		t.Errorf("Expected default iteration 0, got %d", ctx.Iteration())
	}
}

func TestDGAEvaluationContext_WithIteration(t *testing.T) {
	ctx := NewEvaluationContext().WithIteration(5)
	if ctx.Iteration() != 5 {
		t.Errorf("Expected iteration 5, got %d", ctx.Iteration())
	}
}

func TestDGAEvaluationContext_WithIteration_DoesNotModifyOriginal(t *testing.T) {
	original := NewEvaluationContext()
	modified := original.WithIteration(10)

	if original.Iteration() != 0 {
		t.Errorf("Original context iteration changed: got %d, want 0", original.Iteration())
	}
	if modified.Iteration() != 10 {
		t.Errorf("Modified context iteration: got %d, want 10", modified.Iteration())
	}
}

func TestDGAEvaluationContext_Iteration_InAll(t *testing.T) {
	ctx := NewEvaluationContext().WithIteration(7)
	all := ctx.All()
	if v, ok := all["iteration"]; !ok || v != 7 {
		t.Errorf("All()[\"iteration\"] = %v, want 7", v)
	}
}

func TestDGAEvaluationContext_All_WithIterationAndPipeline(t *testing.T) {
	graph := NewDGAGraph()
	pipeline := NewPipeline(nil)
	pipeline.(*PipelineImpl).SetGraph(graph)
	ctx := NewEvaluationContext().WithPipeline(pipeline).WithIteration(3)

	all := ctx.All()
	if all["iteration"] != 3 {
		t.Errorf("iteration = %v, want 3", all["iteration"])
	}
}

func TestLastIndexOf(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   int
	}{
		{"found", "abc.def.ghi", ".", 7},
		{"not found", "abc", ".", -1},
		{"at end", "abc.", ".", 3},
		{"empty string", "", ".", -1},
		{"single match at start", ".abc", ".", 0},
		{"multiple consecutive", "a..b", ".", 2},
		{"longer substring", "abcXYZdefXYZghi", "XYZ", 9},
		{"substr longer than string", "ab", "abc", -1},
		{"empty substr", "abc", "", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastIndexOf(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("lastIndexOf(%q, %q) = %d, want %d", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestLastIndexOfByte(t *testing.T) {
	tests := []struct {
		name string
		s    string
		c    byte
		want int
	}{
		{"found middle", "a.b.c", '.', 3},
		{"not found", "abc", '.', -1},
		{"single char", "a", '.', -1},
		{"empty string", "", '.', -1},
		{"multiple dots", "...", '.', 2},
		{"at start", ".abc", '.', 0},
		{"single match", "abc.def", '.', 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastIndexOfByte(tt.s, tt.c)
			if got != tt.want {
				t.Errorf("lastIndexOfByte(%q, %c) = %d, want %d", tt.s, tt.c, got, tt.want)
			}
		})
	}
}
