//go:build linux

package local

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// prepareCmd 让子进程独立进程组，并把 ctx 取消时的默认终止行为改为**优雅终止**。
//
// 背景：流水线取消/超时需要真正终止节点进程。节点命令经 shell（甚至 script PTY）
// 包裹，默认 CommandContext 只杀直接子进程，孙进程（如 sleep、python）会变孤儿残留。
//
// 取消语义（grace 为零时退化为原行为：立即强杀）：
//  1. 叶子优先向节点进程发 SIGTERM——节点脚本据此优雅退出，回收自己在第三方服务上的
//     后台任务（如推理服务上 running 的 job），避免孤儿任务占 GPU；
//  2. 等待最多 grace；
//  3. 仍未退出则 SIGKILL 整棵树兜底。
//
// 注意：cmd.Cancel 只负责"发起"优雅终止（SIGTERM + 异步升级），返回 nil 表示
// 由 Wait 等待进程真正退出——升级逻辑保证 Wait 不会永久阻塞。
func (l *LocalExecutor) prepareCmd(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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

// interruptProcess 向整个进程组发送 SIGINT（温和中断，保留给交互式场景）
func interruptProcess(pid int) error {
	return syscall.Kill(-pid, syscall.SIGINT)
}

// killProcess 强制终止整棵进程树（含进程组残留兜底）
func killProcess(pid int) error {
	killProcessTree(pid)
	return nil
}

// terminateProcessTreeGracefully 优雅终止整棵进程树：SIGTERM → 等 grace → SIGKILL。
//
// 两个关键点（都由实测反推）：
//
//  1. **先快照整棵树再发信号**：根进程（script/sh）退出后后代会被 reparent 到 init，
//     再也按 ppid 找不回；必须在信号发出前拿住完整 pid 列表，否则宽限期结束时
//     强杀无法覆盖残留。
//  2. **整棵树一起发 SIGTERM，不区分叶子**：节点脚本自己可能用 `trap ... TERM` 做
//     清理（shell 层），只给叶子发会让壳层的 trap 永远收不到信号。PTY 路径下也会
//     因 script/bash 先死而关闭 pty，使节点进程被 SIGHUP 打断——因此节点侧必须同时
//     接管 SIGHUP（见 flowx-pixelforge `flowx_client.py`，已注册 SIGTERM/SIGINT/SIGHUP）。
func terminateProcessTreeGracefully(pid int, grace time.Duration) {
	if pid <= 0 {
		return
	}
	tree := collectTree(pid) // pid → ppid（含根）
	targets := make([]int, 0, len(tree))
	for p := range tree {
		targets = append(targets, p)
	}
	_ = syscall.Kill(-pid, syscall.SIGTERM) // 同进程组兄弟兜底
	for _, p := range targets {
		_ = syscall.Kill(p, syscall.SIGTERM)
	}
	if grace <= 0 {
		killTargets(targets)
		killProcessTree(pid)
		return
	}
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if !anyTargetAlive(targets) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	// 宽限期内仍有进程存活：强杀（快照 pid + 进程组/残留树兜底）
	killTargets(targets)
	killProcessTree(pid)
}

// killTargets 强杀快照里的每个进程（含已 reparent 的残留）
func killTargets(targets []int) {
	for _, t := range targets {
		_ = syscall.Kill(t, syscall.SIGKILL)
	}
}

// anyTargetAlive 快照里是否还有存活进程
func anyTargetAlive(targets []int) bool {
	for _, t := range targets {
		if processAlive(t) {
			return true
		}
	}
	return false
}

// processAlive 判断进程是否仍存活（signal 0 探活 + 排除僵尸）：
// 僵尸进程已死、只等父进程 reap，不应算"仍在运行"，否则优雅终止会白等满宽限期
// （调用方未并发 Wait 时尤其明显）。
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if err := syscall.Kill(pid, 0); err != nil && err != syscall.EPERM {
		return false // ESRCH：进程已不存在
	}
	return !isZombie(pid)
}

// isZombie 读 /proc/<pid>/stat 判断是否僵尸（stat 第 3 字段 = 状态字符）
func isZombie(pid int) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	s := string(data)
	i := strings.LastIndex(s, ")")
	if i < 0 {
		return false
	}
	fields := strings.Fields(s[i+1:])
	return len(fields) > 0 && fields[0] == "Z"
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

// collectTree 解析 /proc/*/stat，返回 pid 及其全部后代的 pid → ppid 映射
func collectTree(pid int) map[int]int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return map[int]int{pid: 0}
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
	tree := map[int]int{pid: ppidOf[pid]}
	seen := map[int]bool{pid: true}
	queue := []int{pid}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for p, pp := range ppidOf {
			if pp == cur && !seen[p] {
				seen[p] = true
				tree[p] = pp
				queue = append(queue, p)
			}
		}
	}
	return tree
}

// collectDescendants 解析 /proc/*/stat 收集 pid 的全部后代进程
func collectDescendants(pid int) []int {
	tree := collectTree(pid)
	out := make([]int, 0, len(tree))
	for p := range tree {
		if p != pid {
			out = append(out, p)
		}
	}
	return out
}
