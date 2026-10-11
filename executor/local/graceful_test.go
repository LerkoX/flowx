//go:build linux

package local

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LerkoX/flowx/executor"
)

// drainUntilClosed 消费 resultChan 直到关闭，返回是否有步骤报错
func drainUntilClosed(t *testing.T, resultChan chan any) {
	t.Helper()
	for data := range resultChan {
		if err, ok := data.(error); ok {
			t.Logf("step error: %v", err)
		}
	}
}

// TestTransfer_CancelGracefulShutdown 上下文取消时先发 SIGTERM：
// 节点脚本捕获信号后应有机会完成清理（回收后台/远程任务），而不是被立即强杀。
func TestTransfer_CancelGracefulShutdown(t *testing.T) {
	exec := NewLocalExecutor()
	exec.setInterruptGrace(5 * time.Second)

	marker := filepath.Join(t.TempDir(), "cleaned")
	// trap SIGTERM → 写标记文件 → 退出；只有收到温和信号才可能执行到这里
	script := `trap 'echo done > ` + marker + `; exit 0' TERM; sleep 30`

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)
	commandChan <- executor.CommandWrapper{StepName: "graceful", Command: script}
	close(commandChan)

	go drainUntilClosed(t, resultChan)
	go func() {
		time.Sleep(200 * time.Millisecond) // 等命令启动并装好 trap
		cancel()
	}()

	start := time.Now()
	exec.Transfer(ctx, resultChan, commandChan, nil)
	elapsed := time.Since(start)

	if elapsed > 4*time.Second {
		t.Errorf("Transfer 等待过久（%v）：应在进程优雅退出后立即返回", elapsed)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("节点脚本未收到 SIGTERM / 未完成清理: %v", err)
	}
}

// TestTransfer_CancelEscalatesToKill 宽限期内不退出的进程必须在宽限期后被 SIGKILL，
// 保证取消不会永久挂住（这也是 SIGTERM 丢失时的兜底）。
func TestTransfer_CancelEscalatesToKill(t *testing.T) {
	exec := NewLocalExecutor()
	exec.setInterruptGrace(300 * time.Millisecond)

	// 忽略 TERM 的进程：只能靠宽限期后的 SIGKILL 结束
	script := `trap '' TERM; sleep 30`

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)
	commandChan <- executor.CommandWrapper{StepName: "stubborn", Command: script}
	close(commandChan)

	go drainUntilClosed(t, resultChan)
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	done := make(chan struct{})
	go func() {
		exec.Transfer(ctx, resultChan, commandChan, nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("宽限期后未强杀：Transfer 未返回")
	}
}

// TestTerminateProcessTreeGracefully_KillsDescendants 祖先进程退出后，宽限期内仍存活的
// 孙进程（Python 节点脚本常见：shell/script 包装）也必须被清理，不能逃逸成孤儿。
func TestTerminateProcessTreeGracefully_KillsDescendants(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "grandchild.pid")
	// 父进程收到 TERM 立即退出，孙进程忽略 TERM 存活 → 依赖收尾清理
	script := `sh -c 'trap "" TERM; sleep 30' & echo $! > ` + pidFile + `; wait`

	exec := NewLocalExecutor()
	exec.setInterruptGrace(300 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultChan := make(chan any, 10)
	commandChan := make(chan any, 1)
	commandChan <- executor.CommandWrapper{StepName: "tree", Command: script}
	close(commandChan)

	go drainUntilClosed(t, resultChan)
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()
	exec.Transfer(ctx, resultChan, commandChan, nil)

	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Skipf("未取到孙进程 pid，跳过: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		t.Skipf("孙进程 pid 解析失败 (%q)", string(raw))
	}
	t.Cleanup(func() { _ = killProcess(pid) })

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("孙进程 %d 未被清理，成为孤儿", pid)
}

// shellQuote 单引号包裹，便于把 python 代码安全塞进 shell 命令
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// pythonNodeProbe 构造一个"节点进程"探针：模拟 flowx_client 的信号契约——
// 同时接管 SIGTERM 与 SIGHUP（PTY 关闭时内核补发 SIGHUP，不接管会被默认动作杀死），
// 收到信号时写标记文件（体外的 pty 已关闭，stdout 不可靠）后退出。
func pythonNodeProbe(t *testing.T, py, marker string) string {
	t.Helper()
	code := strings.Join([]string{
		"import signal,sys,time",
		"h=lambda *a: (open(" + strconv.Quote(marker) + ",'w').write('ok'), sys.exit(143))",
		"signal.signal(signal.SIGTERM, h)",
		"signal.signal(signal.SIGHUP, h)",
		"signal.signal(signal.SIGINT, h)",
		"print('READY', flush=True)",
		"time.sleep(30)",
	}, ";")
	return py + " -c " + shellQuote(code)
}

// runCancelProbe 用执行器跑一段命令，等 READY 后取消，返回 Transfer 耗时
func runCancelProbe(t *testing.T, execu *LocalExecutor, command string) time.Duration {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultChan := make(chan any, 64)
	commandChan := make(chan any, 1)
	commandChan <- executor.CommandWrapper{StepName: "probe", Command: command}
	close(commandChan)

	ready := make(chan struct{})
	var once sync.Once
	go func() {
		for data := range resultChan {
			if b, ok := data.([]byte); ok && strings.Contains(string(b), "READY") {
				once.Do(func() { close(ready) })
			}
		}
	}()
	go func() {
		select {
		case <-ready:
		case <-time.After(5 * time.Second):
		}
		cancel()
	}()
	start := time.Now()
	execu.Transfer(ctx, resultChan, commandChan, nil)
	return time.Since(start)
}

// TestTransfer_CancelGracefulWithPTY PTY（script 包装）下取消也必须把 SIGTERM 送到
// 真正的节点进程并可被处理：session/进程组都会变，只靠 kill(-pid) 打不到 python
// （Studio 默认 local 执行器 pty=true，这条是主力路径）。
func TestTransfer_CancelGracefulWithPTY(t *testing.T) {
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("no script command")
	}
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("no python3")
	}
	marker := filepath.Join(t.TempDir(), "handled")
	execu := NewLocalExecutor()
	execu.setPTY(true)
	execu.setInterruptGrace(5 * time.Second)

	elapsed := runCancelProbe(t, execu, pythonNodeProbe(t, py, marker))

	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("PTY 下节点进程未收到/未处理 SIGTERM（优雅退出失效）: %v", err)
	}
	if elapsed > 4*time.Second {
		t.Errorf("取消等待过久（%v）：应在进程优雅退出后立即返回", elapsed)
	}
}

// TestTransfer_CancelGracefulNoPTY 非 PTY 路径同样必须让节点进程处理 SIGTERM
func TestTransfer_CancelGracefulNoPTY(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("no python3")
	}
	marker := filepath.Join(t.TempDir(), "handled")
	execu := NewLocalExecutor()
	execu.setInterruptGrace(5 * time.Second)

	runCancelProbe(t, execu, pythonNodeProbe(t, py, marker))

	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("非 PTY 下节点进程未处理 SIGTERM: %v", err)
	}
}
