package pipelinex

import "testing"

// mockTemplateEngine 是用于测试的模拟模板引擎
type mockTemplateEngine struct {
	evaluateBoolFn   func(expression string, data map[string]any) (bool, error)
	evaluateStringFn func(expression string, data map[string]any) (string, error)
	validateFn       func(expression string) error
}

func (m *mockTemplateEngine) EvaluateBool(expression string, data map[string]any) (bool, error) {
	if m.evaluateBoolFn != nil {
		return m.evaluateBoolFn(expression, data)
	}
	return false, nil
}

func (m *mockTemplateEngine) EvaluateString(expression string, data map[string]any) (string, error) {
	if m.evaluateStringFn != nil {
		return m.evaluateStringFn(expression, data)
	}
	return "", nil
}

func (m *mockTemplateEngine) Validate(expression string) error {
	if m.validateFn != nil {
		return m.validateFn(expression)
	}
	return nil
}

func TestNewConditionalEdgeWithEngine(t *testing.T) {
	node1 := NewDGANode("node1", "RUNNING")
	node2 := NewDGANode("node2", "UNKNOWN")
	expression := "{{ x == 1 }}"

	called := false
	engine := &mockTemplateEngine{
		evaluateBoolFn: func(expr string, data map[string]any) (bool, error) {
			called = true
			if expr != expression {
				t.Errorf("expression = %q, want %q", expr, expression)
			}
			return true, nil
		},
	}

	edge := NewConditionalEdgeWithEngine(node1, node2, expression, engine)

	if edge.Expression() != expression {
		t.Errorf("Expression() = %q, want %q", edge.Expression(), expression)
	}

	evalCtx := NewEvaluationContext()
	result, err := edge.Evaluate(evalCtx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !result {
		t.Error("Expected true")
	}
	if !called {
		t.Error("Custom engine was not called")
	}
}

func TestNewConditionalEdgeWithEngine_NilEngine(t *testing.T) {
	node1 := NewDGANode("node1", "RUNNING")
	node2 := NewDGANode("node2", "SUCCESS")

	// nil engine 应该回退到默认 Pongo2 引擎
	edge := NewConditionalEdgeWithEngine(node1, node2, "{{ nodeStatus == 'SUCCESS' }}", nil)
	evalCtx := NewEvaluationContext().WithNode(node2)

	result, err := edge.Evaluate(evalCtx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !result {
		t.Error("Expected true with default engine fallback")
	}
}

func TestDGAEdge_SetEngine(t *testing.T) {
	node1 := NewDGANode("node1", "RUNNING")
	node2 := NewDGANode("node2", "UNKNOWN")

	// 创建条件边（engine 为 nil）
	edge := NewConditionalEdge(node1, node2, "{{ true }}")

	// 设置自定义 engine
	called := false
	engine := &mockTemplateEngine{
		evaluateBoolFn: func(expr string, data map[string]any) (bool, error) {
			called = true
			return true, nil
		},
	}
	edge.(*DGAEdge).SetEngine(engine)

	evalCtx := NewEvaluationContext()
	result, err := edge.Evaluate(evalCtx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !result {
		t.Error("Expected true")
	}
	if !called {
		t.Error("SetEngine'd engine was not called")
	}
}

func TestDGAEdge_SetEngine_Overwrites(t *testing.T) {
	node1 := NewDGANode("node1", "RUNNING")
	node2 := NewDGANode("node2", "UNKNOWN")
	edge := NewConditionalEdge(node1, node2, "{{ true }}")

	engineACalled := false
	engineA := &mockTemplateEngine{
		evaluateBoolFn: func(expr string, data map[string]any) (bool, error) {
			engineACalled = true
			return false, nil
		},
	}

	engineBCalled := false
	engineB := &mockTemplateEngine{
		evaluateBoolFn: func(expr string, data map[string]any) (bool, error) {
			engineBCalled = true
			return true, nil
		},
	}

	dgaEdge := edge.(*DGAEdge)
	dgaEdge.SetEngine(engineA)
	dgaEdge.SetEngine(engineB)

	evalCtx := NewEvaluationContext()
	result, err := dgaEdge.Evaluate(evalCtx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !result {
		t.Error("Expected engine B's result (true)")
	}
	if engineACalled {
		t.Error("Engine A should not have been called after overwrite")
	}
	if !engineBCalled {
		t.Error("Engine B should have been called")
	}
}
