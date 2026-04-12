package docker

import (
	"context"
	"testing"

	"github.com/LerkoX/pipelinex/executor"
)

func TestNewDockerExecutorWithClient_NilClient(t *testing.T) {
	exec := NewDockerExecutorWithClient(nil)
	if exec == nil {
		t.Fatal("NewDockerExecutorWithClient(nil) returned nil")
	}
}

func TestNewDockerExecutorWithClient_GetType(t *testing.T) {
	exec := NewDockerExecutorWithClient(nil)
	if exec.GetType() != "docker" {
		t.Errorf("GetType() = %q, want %q", exec.GetType(), "docker")
	}
}

func TestNewDockerExecutorWithClient_GetInstanceId(t *testing.T) {
	exec := NewDockerExecutorWithClient(nil)
	if id := exec.GetInstanceId(); id != "" {
		t.Errorf("GetInstanceId() before Prepare = %q, want empty", id)
	}
}

func TestDockerExecutor_DetectShell(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  string
	}{
		{"alpine", "alpine:latest", "/bin/sh"},
		{"alpine uppercase", "Alpine:3.18", "/bin/sh"},
		{"busybox", "busybox:1.36", "/bin/sh"},
		{"ubuntu", "ubuntu:22.04", "/bin/bash"},
		{"debian", "debian:bookworm", "/bin/bash"},
		{"custom", "myapp:v1", "/bin/bash"},
		{"empty", "", "/bin/bash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewDockerExecutorWithClient(nil)
			exec.image = tt.image
			got := exec.detectShell()
			if got != tt.want {
				t.Errorf("detectShell() with image %q = %q, want %q", tt.image, got, tt.want)
			}
		})
	}
}

func TestDockerExecutor_ResolveImageName(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		registry string
		want     string
	}{
		{"no registry", "alpine:latest", "", "alpine:latest"},
		{"with registry simple", "alpine:latest", "registry.example.com", "registry.example.com/alpine:latest"},
		{"already qualified", "registry.example.com/alpine:latest", "other.registry", "registry.example.com/alpine:latest"},
		{"with slash", "myorg/myapp:v1", "registry.example.com", "myorg/myapp:v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewDockerExecutorWithClient(nil)
			exec.image = tt.image
			exec.registry = tt.registry
			got := exec.resolveImageName()
			if got != tt.want {
				t.Errorf("resolveImageName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDockerExecutor_BuildEnvList(t *testing.T) {
	exec := NewDockerExecutorWithClient(nil)

	// Empty env
	envList := exec.buildEnvList()
	if len(envList) != 0 {
		t.Errorf("Empty env: got %d items, want 0", len(envList))
	}

	// Multiple env vars
	exec.env["KEY1"] = "value1"
	exec.env["KEY2"] = "value2"
	exec.env["KEY3"] = "value3"

	envList = exec.buildEnvList()
	if len(envList) != 3 {
		t.Errorf("3 env vars: got %d items, want 3", len(envList))
	}

	// 验证格式 KEY=VALUE
	found := make(map[string]bool)
	for _, e := range envList {
		found[e] = true
	}
	if !found["KEY1=value1"] || !found["KEY2=value2"] || !found["KEY3=value3"] {
		t.Errorf("Env list missing expected entries: %v", envList)
	}
}

func TestDockerExecutor_BuildMounts(t *testing.T) {
	exec := NewDockerExecutorWithClient(nil)

	// Empty volumes
	mounts := exec.buildMounts()
	if len(mounts) != 0 {
		t.Errorf("Empty volumes: got %d mounts, want 0", len(mounts))
	}

	// Multiple volumes
	exec.volumes["/host/path1"] = "/container/path1"
	exec.volumes["/host/path2"] = "/container/path2"

	mounts = exec.buildMounts()
	if len(mounts) != 2 {
		t.Errorf("2 volumes: got %d mounts, want 2", len(mounts))
	}
}

func TestDockerExecutor_CopyToContainer_NotPrepared(t *testing.T) {
	exec := NewDockerExecutorWithClient(nil)
	err := exec.copyToContainer(context.Background(), "/local", "/container")
	if err == nil {
		t.Error("Expected error when container not prepared")
	}
}

func TestDockerExecutor_CopyFromContainer_NotPrepared(t *testing.T) {
	exec := NewDockerExecutorWithClient(nil)
	err := exec.copyFromContainer(context.Background(), "/container", "/local")
	if err == nil {
		t.Error("Expected error when container not prepared")
	}
}

func TestDockerExecutor_GetRuntimeInfo_BeforePrepare(t *testing.T) {
	exec := NewDockerExecutorWithClient(nil)
	exec.image = "alpine:latest"
	exec.workdir = "/app"
	exec.network = "host"
	exec.registry = "hub.rat.dev"

	info := exec.GetRuntimeInfo()
	if info == nil {
		t.Fatal("GetRuntimeInfo() returned nil")
	}
	if info["containerId"] != "" {
		t.Errorf("containerId = %v, want empty", info["containerId"])
	}
	if info["image"] != "alpine:latest" {
		t.Errorf("image = %v, want 'alpine:latest'", info["image"])
	}
	if info["workdir"] != "/app" {
		t.Errorf("workdir = %v, want '/app'", info["workdir"])
	}
	if info["network"] != "host" {
		t.Errorf("network = %v, want 'host'", info["network"])
	}
	if info["registry"] != "hub.rat.dev" {
		t.Errorf("registry = %v, want 'hub.rat.dev'", info["registry"])
	}
}

// 接口实现检查
func TestDockerExecutor_InterfaceCompliance(t *testing.T) {
	var _ executor.Executor = (*DockerExecutor)(nil)
}
