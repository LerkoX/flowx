package docker

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/errdefs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blackholeDaemon 假 daemon：只接受 TCP 连接、永不回响应。
// 复现 exec 407 事故现场 —— host 端口能连上（隧道断开后中间设备仍接受连接），
// 但 daemon 不响应；没有响应头超时时 docker client 会永久挂起。
type blackholeDaemon struct {
	ln    net.Listener
	mu    sync.Mutex
	conns []net.Conn
	done  chan struct{}
}

func newBlackholeDaemon(t *testing.T) *blackholeDaemon {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	d := &blackholeDaemon{ln: ln, done: make(chan struct{})}
	go func() {
		defer close(d.done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			d.mu.Lock()
			d.conns = append(d.conns, conn)
			d.mu.Unlock()
			// 不读不写：让客户端一直等响应头
		}
	}()

	t.Cleanup(func() {
		_ = ln.Close()
		d.mu.Lock()
		for _, c := range d.conns {
			_ = c.Close()
		}
		d.mu.Unlock()
		<-d.done
	})
	return d
}

func (d *blackholeDaemon) host() string { return "tcp://" + d.ln.Addr().String() }

// TestDockerExecutor_UnresponsiveDaemonFailsFast 验证 daemon 不响应时 Prepare 快速失败，
// 而不是永久阻塞（exec 407：节点一直停在 running，整条流水线卡死）。
func TestDockerExecutor_UnresponsiveDaemonFailsFast(t *testing.T) {
	daemon := newBlackholeDaemon(t)

	exec, err := NewDockerExecutor()
	require.NoError(t, err)
	require.NoError(t, applyConfigToExecutor(map[string]any{
		"host":          daemon.host(),
		"image":         "alpine:latest",
		"daemonTimeout": "500ms",
	}, exec))

	start := time.Now()
	err = exec.Prepare(context.Background()) // 无 deadline，模拟 engine 调用
	elapsed := time.Since(start)

	require.Error(t, err, "daemon 不响应时 Prepare 必须失败")
	assert.Less(t, elapsed, 10*time.Second, "必须在超时后快速失败，实际耗时 %s", elapsed)
	assert.Contains(t, err.Error(), "not responding",
		"错误应指出 daemon 不可达，实际: %v", err)
}

// TestDockerExecutor_TestConnectionUnresponsiveDaemon 验证连接测试同样有界。
func TestDockerExecutor_TestConnectionUnresponsiveDaemon(t *testing.T) {
	daemon := newBlackholeDaemon(t)

	exec, err := NewDockerExecutor()
	require.NoError(t, err)
	require.NoError(t, applyConfigToExecutor(map[string]any{
		"host":          daemon.host(),
		"daemonTimeout": "500ms",
	}, exec))

	start := time.Now()
	_, err = exec.TestConnection(context.Background())
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 10*time.Second, "连接测试必须有界，实际耗时 %s", elapsed)
}

// TestDockerExecutor_DaemonResponseTimeout 验证默认值与显式配置。
func TestDockerExecutor_DaemonResponseTimeout(t *testing.T) {
	exec, err := NewDockerExecutor()
	require.NoError(t, err)
	assert.Equal(t, defaultDaemonTimeout, exec.daemonResponseTimeout())

	exec.setDaemonTimeout(0)
	assert.Equal(t, defaultDaemonTimeout, exec.daemonResponseTimeout())

	exec.setDaemonTimeout(2 * time.Second)
	assert.Equal(t, 2*time.Second, exec.daemonResponseTimeout())
}

// TestApplyConfigToExecutor_DaemonTimeout 验证 daemonTimeout 支持字符串时长与秒数。
func TestApplyConfigToExecutor_DaemonTimeout(t *testing.T) {
	exec, err := NewDockerExecutor()
	require.NoError(t, err)
	require.NoError(t, applyConfigToExecutor(map[string]any{"daemonTimeout": "20s"}, exec))
	assert.Equal(t, 20*time.Second, exec.daemonResponseTimeout())

	exec2, err := NewDockerExecutor()
	require.NoError(t, err)
	require.NoError(t, applyConfigToExecutor(map[string]any{"daemonTimeout": 7}, exec2))
	assert.Equal(t, 7*time.Second, exec2.daemonResponseTimeout())
}

// TestIsImageNotFound 验证只有"镜像不存在"才转入拉取，
// daemon 不可达/超时不能被误判为镜像缺失。
func TestIsImageNotFound(t *testing.T) {
	assert.True(t, isImageNotFound(errdefs.NotFound(errors.New("Error response from daemon: No such image: foo"))))
	assert.True(t, isImageNotFound(errors.New("Error response from daemon: no such image: foo")))
	assert.False(t, isImageNotFound(context.DeadlineExceeded))
	assert.False(t, isImageNotFound(errors.New("Cannot connect to the Docker daemon")))
	// 隧道/代理返回的 404 页面不能当成"镜像缺失"（否则会误入拉取分支掩盖连接问题）
	assert.False(t, isImageNotFound(errors.New("404 page not found")))
	assert.False(t, isImageNotFound(nil))
}
