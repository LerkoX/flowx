//go:build !linux

package local

import (
	"os"
	"os/exec"
	"time"
)

// prepareCmd 非 Linux 平台：把 ctx 取消时的终止行为改为优雅终止（先 Interrupt，
// 宽限期后 Kill）。语义与 linux 版一致，见 procgroup_linux.go 注释。
func (l *LocalExecutor) prepareCmd(cmd *exec.Cmd) *exec.Cmd {
	grace := l.interruptGraceValue()
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		terminateProcessTreeGracefully(cmd.Process.Pid, grace)
		return nil
	}
	return cmd
}

// interruptProcess 发送中断信号（Windows 上为 Ctrl+Break 的近似）
func interruptProcess(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(os.Interrupt)
}

// killProcess 强制终止单个进程（非 Linux 无进程树概念）
func killProcess(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

// terminateProcessTreeGracefully 优雅终止：Interrupt → 等 grace → Kill
// （非 Linux 无 /proc 进程树信息，语义从简，见 procgroup_linux.go）
func terminateProcessTreeGracefully(pid int, grace time.Duration) {
	if pid <= 0 {
		return
	}
	_ = interruptProcess(pid)
	if grace <= 0 {
		_ = killProcess(pid)
		return
	}
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = killProcess(pid)
}

// processAlive 判断进程是否仍存在（非 Linux 无 signal 0 探活，按 FindProcess 结果近似）
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// 非阻塞探测：Interrupt 之外的信号不适用，用 Kill(0) 语义的平台从简——
	// 直接视为存活，最终由宽限期后的 Kill 兜底。
	_ = p
	return true
}
