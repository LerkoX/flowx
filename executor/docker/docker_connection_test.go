package docker

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestConnectionWithConfig_InvalidHost 不可达 host 应返回带原因的 error
func TestConnectionWithConfig_InvalidHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := TestConnectionWithConfig(ctx, map[string]any{"host": "tcp://127.0.0.1:1"})
	if err == nil {
		t.Fatal("expected connection error for unreachable host")
	}
	if !strings.Contains(err.Error(), "ping failed") {
		t.Fatalf("error should mention ping failure, got: %v", err)
	}
}

// TestConnection_LocalDaemon 本机 daemon 可用时应返回版本信息（无 daemon 时跳过）
func TestConnection_LocalDaemon(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := TestConnectionWithConfig(ctx, map[string]any{})
	if err != nil {
		t.Skipf("no local docker daemon available: %v", err)
	}
	if info.ServerVersion == "" {
		t.Fatal("expected non-empty server version")
	}
	if info.LatencyMs < 0 {
		t.Fatalf("unexpected latency: %d", info.LatencyMs)
	}
}
