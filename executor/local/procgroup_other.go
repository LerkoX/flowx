//go:build !linux

package local

import (
	"os"
	"os/exec"
)

// prepareCmd 非 Linux 平台保持默认行为（仅杀直接子进程）
func prepareCmd(cmd *exec.Cmd) *exec.Cmd {
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

// killProcess 强制终止单个进程
func killProcess(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
