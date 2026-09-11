//go:build linux

package local

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// prepareCmd 让子进程独立进程组，并把 ctx 取消时的默认终止行为改为杀整棵进程树。
//
// 背景：流水线取消/超时需要真正终止节点进程。节点命令经 shell（甚至 script PTY）
// 包裹，默认 CommandContext 只杀直接子进程，孙进程（如 sleep、python）会变孤儿残留。
func prepareCmd(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		killProcessTree(cmd.Process.Pid)
		return nil
	}
	return cmd
}

// interruptProcess 向整个进程组发送 SIGINT（温和中断）
func interruptProcess(pid int) error {
	return syscall.Kill(-pid, syscall.SIGINT)
}

// killProcess 强制终止整棵进程树（含进程组残留兜底）
func killProcess(pid int) error {
	killProcessTree(pid)
	return nil
}

// killProcessTree 先杀所有子孙进程再杀根进程；同进程组残留兜底。
// 进程树通过 /proc 的 ppid 关系收集，不依赖进程组（script PTY 会新建会话，
// 孙进程可能不在同一进程组）。
func killProcessTree(pid int) {
	for _, child := range collectDescendants(pid) {
		_ = syscall.Kill(child, syscall.SIGKILL)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

// collectDescendants 解析 /proc/*/stat 收集 pid 的全部后代进程
func collectDescendants(pid int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	ppidOf := make(map[int]int, len(entries))
	for _, e := range entries {
		p, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		s := string(data)
		// stat 格式: pid (comm) state ppid ...；comm 可含空格，取最后一个 ')' 之后
		i := strings.LastIndex(s, ")")
		if i < 0 {
			continue
		}
		fields := strings.Fields(s[i+1:])
		if len(fields) < 2 {
			continue
		}
		pp, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		ppidOf[p] = pp
	}
	set := map[int]bool{pid: true}
	var out []int
	for changed := true; changed; {
		changed = false
		for p, pp := range ppidOf {
			if set[pp] && !set[p] {
				set[p] = true
				out = append(out, p)
				changed = true
			}
		}
	}
	return out
}
