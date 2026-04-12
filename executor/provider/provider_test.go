package provider

import (
	"context"
	"testing"
)

func TestNewProvider(t *testing.T) {
	p := NewProvider()
	if p == nil {
		t.Fatal("NewProvider() returned nil")
	}
	if p.executorConfigs == nil {
		t.Error("executorConfigs map should be initialized")
	}
}

func TestProvider_GetExecutor_NotFound(t *testing.T) {
	p := NewProvider()
	_, err := p.GetExecutor(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for unregistered executor")
	}
}

func TestProvider_GetExecutor_UnsupportedType(t *testing.T) {
	p := NewProvider()
	p.RegisterExecutor("bad", ExecutorConfig{Type: "foobar", Config: map[string]any{}})

	_, err := p.GetExecutor(context.Background(), "bad")
	if err == nil {
		t.Error("Expected error for unsupported executor type")
	}
}

func TestProvider_GetExecutor_LocalType(t *testing.T) {
	p := NewProvider()
	p.RegisterExecutor("local-exec", ExecutorConfig{
		Type: "local",
		Config: map[string]any{
			"shell": "bash",
		},
	})

	exec, err := p.GetExecutor(context.Background(), "local-exec")
	if err != nil {
		t.Fatalf("GetExecutor() error = %v", err)
	}
	if exec == nil {
		t.Fatal("GetExecutor() returned nil executor")
	}

	// 清理
	exec.Destruction(context.Background())
}

func TestProvider_GetExecutor_LocalWithInvalidTimeout_IgnoresGracefully(t *testing.T) {
	// timeout 配置错误时，applyConfigToExecutor 静默忽略，不会报错
	p := NewProvider()
	p.RegisterExecutor("local-timeout", ExecutorConfig{
		Type: "local",
		Config: map[string]any{
			"timeout": "invalid-duration",
		},
	})

	exec, err := p.GetExecutor(context.Background(), "local-timeout")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	exec.Destruction(context.Background())
}

func TestProvider_RegisterExecutor_MultipleExecutors(t *testing.T) {
	p := NewProvider()
	p.RegisterExecutor("local-a", ExecutorConfig{
		Type:   "local",
		Config: map[string]any{},
	})
	p.RegisterExecutor("local-b", ExecutorConfig{
		Type:   "local",
		Config: map[string]any{"shell": "sh"},
	})

	execA, err := p.GetExecutor(context.Background(), "local-a")
	if err != nil {
		t.Fatalf("GetExecutor('local-a') error = %v", err)
	}
	execA.Destruction(context.Background())

	execB, err := p.GetExecutor(context.Background(), "local-b")
	if err != nil {
		t.Fatalf("GetExecutor('local-b') error = %v", err)
	}
	execB.Destruction(context.Background())
}

func TestProvider_RegisterExecutor_Overwrite(t *testing.T) {
	p := NewProvider()
	p.RegisterExecutor("my-exec", ExecutorConfig{Type: "foobar", Config: map[string]any{}})
	_, err := p.GetExecutor(context.Background(), "my-exec")
	if err == nil {
		t.Error("Expected error for unsupported type foobar")
	}

	// 覆盖注册
	p.RegisterExecutor("my-exec", ExecutorConfig{Type: "local", Config: map[string]any{}})
	exec, err := p.GetExecutor(context.Background(), "my-exec")
	if err != nil {
		t.Fatalf("GetExecutor() after overwrite error = %v", err)
	}
	exec.Destruction(context.Background())
}
