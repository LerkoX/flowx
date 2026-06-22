package docker

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LerkoX/flowx/executor"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// skipIfNoDocker 在没有 Docker 的环境中跳过测试
func skipIfNoDocker(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !isDockerAvailable(ctx) {
		t.Skip("Docker is not available, skipping test")
	}
}

// setupDockerExecutor 创建一个用于测试的 DockerExecutor
func setupDockerExecutor(t *testing.T, ctx context.Context, image string) *DockerExecutor {
	t.Helper()
	exec, err := NewDockerExecutor()
	require.NoError(t, err)
	exec.setImage(image)
	require.NoError(t, exec.Prepare(ctx))
	return exec
}

// cleanupExecutor 清理执行器并关闭 Docker client，避免 goroutine 泄漏
func cleanupExecutor(t *testing.T, ctx context.Context, exec *DockerExecutor) {
	t.Helper()
	_ = exec.Destruction(ctx)
	_ = exec.client.Close()
}

// TestDockerExecutor_Transfer_GoroutineNoLeak 验证 Transfer 正常结束时没有 goroutine 泄漏
func TestDockerExecutor_Transfer_GoroutineNoLeak(t *testing.T) {
	skipIfNoDocker(t)
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	exec := setupDockerExecutor(t, ctx, "hub.rat.dev/alpine:latest")
	defer cleanupExecutor(t, ctx, exec)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan, nil)

	commandChan <- executor.CommandWrapper{StepName: "step1", Command: "echo hello"}
	close(commandChan)

	drainResults(resultChan, 5*time.Second)

	// 等待 Transfer 内部 goroutine 退出
	time.Sleep(200 * time.Millisecond)
}

// TestDockerExecutor_ExecuteCommand_InteractiveInput 验证交互式输入能正确写入容器进程
func TestDockerExecutor_ExecuteCommand_InteractiveInput(t *testing.T) {
	skipIfNoDocker(t)
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	exec := setupDockerExecutor(t, ctx, "hub.rat.dev/alpine:latest")
	exec.setTTY(true)
	defer cleanupExecutor(t, ctx, exec)

	resultChan := make(chan any, 20)
	commandChan := make(chan any, 1)
	inputChan := make(chan []byte, 5)

	go exec.Transfer(ctx, resultChan, commandChan, inputChan)

	commandChan <- executor.CommandWrapper{
		StepName: "interactive",
		Command:  "read -r name; echo \"Hello $name\"",
	}

	// 等待命令启动并读取提示（TTY 模式下通常没有可见提示，直接发输入）
	time.Sleep(300 * time.Millisecond)
	inputChan <- []byte("Alice\n")

	var output bytes.Buffer
	var stepResult *executor.StepResult

	timeout := time.After(10 * time.Second)
waitLoop:
	for {
		select {
		case res := <-resultChan:
			switch v := res.(type) {
			case []byte:
				output.Write(v)
			case *executor.StepResult:
				stepResult = v
				break waitLoop
			}
		case <-timeout:
			break waitLoop
		}
	}

	require.NotNil(t, stepResult, "should receive StepResult")
	assert.NoError(t, stepResult.Error, "interactive command should succeed")
	assert.Contains(t, output.String(), "Hello Alice", "input should be passed to container process")
}

// TestDockerExecutor_ExecuteCommand_TTYOutputNoFrameHeader 验证 TTY 模式下输出不含 Docker multiplex frame header
func TestDockerExecutor_ExecuteCommand_TTYOutputNoFrameHeader(t *testing.T) {
	skipIfNoDocker(t)
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	exec := setupDockerExecutor(t, ctx, "hub.rat.dev/alpine:latest")
	exec.setTTY(true)
	defer cleanupExecutor(t, ctx, exec)

	resultChan := make(chan any, 20)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan, nil)

	commandChan <- executor.CommandWrapper{StepName: "tty-output", Command: "echo FLOWX_MARKER"}
	close(commandChan)

	var output bytes.Buffer
	drainWithCallback(resultChan, 10*time.Second, func(res any) bool {
		if data, ok := res.([]byte); ok {
			output.Write(data)
		}
		return false
	})

	outStr := output.String()
	assert.Contains(t, outStr, "FLOWX_MARKER")
	// Docker multiplex header 在 Tty=false 时出现，TTY 模式下不应包含 0x01/0x02 开头 + size 的 8 字节 header
	assert.NotContains(t, outStr, "\x01\x00\x00\x00\x00\x00\x00")
	assert.NotContains(t, outStr, "\x02\x00\x00\x00\x00\x00\x00")
}

// TestDockerExecutor_ExecuteCommand_OutputBufferNotReused 验证输出回调的数据是复制后的，不会被后续扫描覆盖
func TestDockerExecutor_ExecuteCommand_OutputBufferNotReused(t *testing.T) {
	skipIfNoDocker(t)
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	exec := setupDockerExecutor(t, ctx, "hub.rat.dev/alpine:latest")
	exec.setTTY(true)
	defer cleanupExecutor(t, ctx, exec)

	resultChan := make(chan any, 100)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan, nil)

	commandChan <- executor.CommandWrapper{StepName: "seq", Command: "seq -w 1 50"}
	close(commandChan)

	var lines []string
	var mu sync.Mutex
	drainWithCallback(resultChan, 15*time.Second, func(res any) bool {
		if data, ok := res.([]byte); ok {
			mu.Lock()
			lines = append(lines, strings.TrimSpace(string(data)))
			mu.Unlock()
		}
		return false
	})

	mu.Lock()
	got := strings.Join(lines, "\n")
	mu.Unlock()

	for i := 1; i <= 50; i++ {
		expected := fmt.Sprintf("%02d", i)
		assert.Contains(t, got, expected, "output should contain line %s", expected)
	}
}

// TestDockerExecutor_ExecuteCommand_ResultChanClosedSafeSend 验证 resultChan 关闭后不会 panic
func TestDockerExecutor_ExecuteCommand_ResultChanClosedSafeSend(t *testing.T) {
	skipIfNoDocker(t)
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	exec := setupDockerExecutor(t, ctx, "hub.rat.dev/alpine:latest")
	exec.setTTY(true)
	defer cleanupExecutor(t, ctx, exec)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan, nil)

	commandChan <- executor.CommandWrapper{StepName: "echo", Command: "echo hi"}
	close(commandChan)

	// 立即关闭 resultChan，模拟调用方异常行为
	time.Sleep(100 * time.Millisecond)
	close(resultChan)

	// 给 Transfer 一点时间尝试发送后续结果，不应 panic
	time.Sleep(300 * time.Millisecond)
}

// TestDockerExecutor_ExecuteCommand_ContextCancelTerminates 验证上下文取消能终止当前命令
func TestDockerExecutor_ExecuteCommand_ContextCancelTerminates(t *testing.T) {
	skipIfNoDocker(t)
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	ctx, cancel := context.WithCancel(context.Background())

	exec := setupDockerExecutor(t, ctx, "hub.rat.dev/alpine:latest")
	exec.setTTY(true)
	defer cleanupExecutor(t, context.Background(), exec)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan, nil)

	commandChan <- executor.CommandWrapper{StepName: "long-running", Command: "sleep 100"}

	// 等待命令启动
	time.Sleep(1 * time.Second)
	cancel()

	// 等待取消传播
	time.Sleep(1 * time.Second)
}

// TestDockerExecutor_Prepare_ContainerExitsImmediately 验证容器启动后立即退出会返回错误
func TestDockerExecutor_Prepare_ContainerExitsImmediately(t *testing.T) {
	skipIfNoDocker(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	exec, err := NewDockerExecutor()
	require.NoError(t, err)
	defer exec.client.Close()

	// 直接创建一个立刻退出的容器来测试 waitForContainer 分支
	containerName := fmt.Sprintf("flowx-exit-test-%d", time.Now().UnixNano())
	resp, err := exec.client.ContainerCreate(ctx, &container.Config{
		Image: "hub.rat.dev/alpine:latest",
		Cmd:   []string{"false"},
	}, &container.HostConfig{}, nil, nil, containerName)
	require.NoError(t, err)
	defer exec.client.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})

	err = exec.client.ContainerStart(ctx, resp.ID, container.StartOptions{})
	require.NoError(t, err)

	// 等待容器退出
	time.Sleep(500 * time.Millisecond)

	exec.mu.Lock()
	exec.containerID = resp.ID
	exec.mu.Unlock()

	err = exec.waitForContainer(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exited with code")
}

// TestDockerExecutor_ExecuteCommand_TTYSizeApplied 验证 TTY 尺寸通过 Docker API 生效
func TestDockerExecutor_ExecuteCommand_TTYSizeApplied(t *testing.T) {
	skipIfNoDocker(t)
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	exec := setupDockerExecutor(t, ctx, "hub.rat.dev/alpine:latest")
	exec.setTTY(true)
	exec.setTTYSize(60, 25)
	defer cleanupExecutor(t, ctx, exec)

	resultChan := make(chan any, 20)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan, nil)

	// 使用 tput 验证终端尺寸，比 stty size 更稳定
	commandChan <- executor.CommandWrapper{
		StepName: "tty-size",
		Command:  "tput lines; tput cols",
	}
	close(commandChan)

	var output bytes.Buffer
	drainWithCallback(resultChan, 10*time.Second, func(res any) bool {
		if data, ok := res.([]byte); ok {
			output.Write(data)
		}
		return false
	})

	// 提取数字并验证
	re := regexp.MustCompile(`\d+`)
	nums := re.FindAllString(output.String(), -1)
	require.GreaterOrEqual(t, len(nums), 2, "should output lines and cols, got: %q", output.String())
	assert.Equal(t, "25", nums[0], "lines should be 25")
	assert.Equal(t, "60", nums[1], "cols should be 60")
}

// TestDockerExecutor_ExecuteCommand_InputRequestEvent 验证 flowx-input 代码块能触发 InputRequestEvent
func TestDockerExecutor_ExecuteCommand_InputRequestEvent(t *testing.T) {
	skipIfNoDocker(t)
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	exec := setupDockerExecutor(t, ctx, "hub.rat.dev/alpine:latest")
	exec.setTTY(true)
	defer cleanupExecutor(t, ctx, exec)

	resultChan := make(chan any, 20)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan, nil)

	commandChan <- executor.CommandWrapper{
		StepName: "input-request",
		Command:  "printf '```flowx-input\ntype: text\nprompt: Please enter your name\n```\n'",
	}
	close(commandChan)

	var event *executor.InputRequestEvent
	drainWithCallback(resultChan, 10*time.Second, func(res any) bool {
		if ev, ok := res.(*executor.InputRequestEvent); ok {
			event = ev
			return true
		}
		return false
	})

	require.NotNil(t, event, "should receive InputRequestEvent")
	assert.Equal(t, "input-request", event.StepName)
	require.NotNil(t, event.Request)
	assert.Equal(t, "text", event.Request.Type)
	assert.Equal(t, "Please enter your name", event.Request.Prompt)
}

// TestDockerExecutor_ExecuteCommand_CommandFailure 验证命令非零退出码会返回错误
func TestDockerExecutor_ExecuteCommand_CommandFailure(t *testing.T) {
	skipIfNoDocker(t)
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	exec := setupDockerExecutor(t, ctx, "hub.rat.dev/alpine:latest")
	exec.setTTY(true)
	defer cleanupExecutor(t, ctx, exec)

	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)

	go exec.Transfer(ctx, resultChan, commandChan, nil)

	commandChan <- executor.CommandWrapper{StepName: "failing", Command: "exit 7"}
	close(commandChan)

	var stepResult *executor.StepResult
	drainWithCallback(resultChan, 10*time.Second, func(res any) bool {
		if sr, ok := res.(*executor.StepResult); ok {
			stepResult = sr
			return true
		}
		return false
	})

	require.NotNil(t, stepResult)
	require.Error(t, stepResult.Error)
	assert.Contains(t, stepResult.Error.Error(), "7")
}

// drainResults  drain resultChan 直到关闭或超时
func drainResults(ch <-chan any, timeout time.Duration) {
	timeoutCh := time.After(timeout)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-timeoutCh:
			return
		}
	}
}

// drainWithCallback drain resultChan，对每个结果调用回调；当回调返回 true 时提前结束
func drainWithCallback(ch <-chan any, timeout time.Duration, cb func(any) bool) {
	timeoutCh := time.After(timeout)
	for {
		select {
		case res, ok := <-ch:
			if !ok {
				return
			}
			if cb(res) {
				return
			}
		case <-timeoutCh:
			return
		}
	}
}
