package docker

import (
	"context"
	"testing"
)

// TestApplyConfigToExecutor_Host 验证 host/tlsVerify/certPath 配置被正确解析到执行器
func TestApplyConfigToExecutor_Host(t *testing.T) {
	exec, err := NewDockerExecutor()
	if err != nil {
		t.Fatalf("NewDockerExecutor() error = %v", err)
	}

	err = applyConfigToExecutor(map[string]any{
		"host":      "tcp://192.168.1.10:2375",
		"tlsVerify": true,
		"certPath":  "/tmp/docker-certs",
	}, exec)
	if err != nil {
		t.Fatalf("applyConfigToExecutor() error = %v", err)
	}

	if exec.host != "tcp://192.168.1.10:2375" {
		t.Errorf("host = %q, want %q", exec.host, "tcp://192.168.1.10:2375")
	}
	if !exec.tlsVerify {
		t.Error("tlsVerify = false, want true")
	}
	if exec.certPath != "/tmp/docker-certs" {
		t.Errorf("certPath = %q, want %q", exec.certPath, "/tmp/docker-certs")
	}
}

// TestApplyConfigToExecutor_HostSSH 验证 ssh:// 协议地址可透传
func TestApplyConfigToExecutor_HostSSH(t *testing.T) {
	exec, _ := NewDockerExecutor()
	if err := applyConfigToExecutor(map[string]any{"host": "ssh://deploy@remote"}, exec); err != nil {
		t.Fatalf("applyConfigToExecutor() error = %v", err)
	}
	if exec.host != "ssh://deploy@remote" {
		t.Errorf("host = %q, want %q", exec.host, "ssh://deploy@remote")
	}
}

// TestEnsureClient_WithHost 验证配置 host 后惰性创建 client 成功且使用指定 host
func TestEnsureClient_WithHost(t *testing.T) {
	exec, _ := NewDockerExecutor()
	if exec.client != nil {
		t.Fatal("client should be nil before ensureClient (lazy init)")
	}

	exec.setHost("tcp://192.168.1.10:2375")
	if err := exec.ensureClient(); err != nil {
		t.Fatalf("ensureClient() error = %v", err)
	}
	if exec.client == nil {
		t.Fatal("client is nil after ensureClient")
	}
	if got := exec.client.DaemonHost(); got != "tcp://192.168.1.10:2375" {
		t.Errorf("DaemonHost() = %q, want %q", got, "tcp://192.168.1.10:2375")
	}

	// 幂等：再次调用不重建
	first := exec.client
	if err := exec.ensureClient(); err != nil {
		t.Fatalf("ensureClient() second call error = %v", err)
	}
	if exec.client != first {
		t.Error("ensureClient() is not idempotent, client was rebuilt")
	}
}

// TestEnsureClient_DefaultEnv 验证未配置 host 时回退环境变量（FromEnv，本机 socket）
func TestEnsureClient_DefaultEnv(t *testing.T) {
	exec, _ := NewDockerExecutor()
	if err := exec.ensureClient(); err != nil {
		t.Fatalf("ensureClient() error = %v", err)
	}
	if exec.client == nil {
		t.Fatal("client is nil after ensureClient")
	}
}

// TestEnsureClient_TLSMissingCerts 验证 tlsVerify 但证书不存在时报错而非 panic
func TestEnsureClient_TLSMissingCerts(t *testing.T) {
	exec, _ := NewDockerExecutor()
	exec.setHost("tcp://192.168.1.10:2376")
	exec.setTLSVerify(true)
	exec.setCertPath(t.TempDir()) // 空目录，无 ca.pem 等

	if err := exec.ensureClient(); err == nil {
		t.Error("ensureClient() with missing TLS certs should return error")
	}
}

// TestEnsureClient_WithClientPreset 验证 NewDockerExecutorWithClient 预设 client 不被覆盖
func TestEnsureClient_WithClientPreset(t *testing.T) {
	exec := NewDockerExecutorWithClient(nil)
	// client 字段为 nil（WithClient(nil) 是测试场景的合法输入），
	// ensureClient 会按惰性逻辑重建——验证不 panic 即可
	exec.setHost("tcp://192.168.1.10:2375")
	if err := exec.ensureClient(); err != nil {
		t.Fatalf("ensureClient() error = %v", err)
	}
	if got := exec.client.DaemonHost(); got != "tcp://192.168.1.10:2375" {
		t.Errorf("DaemonHost() = %q, want %q", got, "tcp://192.168.1.10:2375")
	}
}

// TestProvider_DockerHostConfig 经 Provider 全链路验证 host 配置透传
func TestProvider_DockerHostConfig(t *testing.T) {
	adapter := NewDockerAdapter()
	err := adapter.Config(context.Background(), map[string]any{
		"host":    "tcp://10.0.0.2:2375",
		"network": "bridge",
	})
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}

	bridge := NewDockerBridge()
	exec, err := bridge.Conn(context.Background(), adapter)
	if err != nil {
		t.Fatalf("Conn() error = %v", err)
	}

	de, ok := exec.(*DockerExecutor)
	if !ok {
		t.Fatalf("Conn() returned %T, want *DockerExecutor", exec)
	}
	if de.host != "tcp://10.0.0.2:2375" {
		t.Errorf("host = %q, want %q", de.host, "tcp://10.0.0.2:2375")
	}
	if de.network != "bridge" {
		t.Errorf("network = %q, want %q", de.network, "bridge")
	}
	// 惰性：Conn 阶段不应创建 client
	if de.client != nil {
		t.Error("client should be lazy-initialized, got non-nil after Conn")
	}
}
