package docker

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LerkoX/flowx/executor"
	"github.com/docker/docker/client"
)

// 断流处置判定（纯函数，无需 docker daemon）：
// 仅当进程仍在运行时才视为"流被截断"——命令正常结束时进程必然已退出。
func TestClassifyStreamEnd(t *testing.T) {
	cases := []struct {
		name       string
		running    bool
		reattached int
		max        int
		want       streamOutcome
	}{
		{"正常结束", false, 0, 3, streamOutcomeDone},
		{"进程仍在运行首轮截断", true, 0, 3, streamOutcomeReattach},
		{"进程仍在运行第二轮截断", true, 1, 3, streamOutcomeReattach},
		{"用尽重挂次数", true, 3, 3, streamOutcomeGiveUp},
		{"进程已退出优先判正常结束（即使已重挂过）", false, 2, 3, streamOutcomeDone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyStreamEnd(tc.running, tc.reattached, tc.max); got != tc.want {
				t.Fatalf("classifyStreamEnd(%v,%d,%d) = %v, want %v",
					tc.running, tc.reattached, tc.max, got, tc.want)
			}
		})
	}
}

func TestExecStreamReattachBackoff(t *testing.T) {
	want := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second}
	for i, w := range want {
		if got := execStreamReattachBackoff(i + 1); got != w {
			t.Errorf("backoff(%d) = %v, want %v", i+1, got, w)
		}
	}
	// 边界：0/负数与超限都收敛到合法退避
	if got := execStreamReattachBackoff(0); got != 500*time.Millisecond {
		t.Errorf("backoff(0) = %v, want 500ms", got)
	}
	if got := execStreamReattachBackoff(99); got != 2*time.Second {
		t.Errorf("backoff(99) = %v, want 2s", got)
	}
}

// ---------- 假 docker daemon：可编程的 exec attach/场景 ----------

// fakeDaemon 用 httptest 模拟 docker daemon 的 exec 端点：
//   - POST .../exec/{id}/start → 101 升级后按 rounds 逐轮写 stdout 帧再关闭连接
//     （关闭即"流结束"：正常结束与断流在协议层完全同形，只能靠 inspect 区分）
//   - GET  .../exec/{id}/json  → 按 inspectSequence 依次返回 Running/ExitCode
type fakeDaemon struct {
	mu              sync.Mutex
	rounds          [][]string // 每轮 attach 输出的行
	roundIdx        int
	inspectSequence []bool // 显式覆盖 Running 序列（用尽后沿用最后一个）
	inspectIdx      int
	// exitAfterAttach：第 N 次 attach 流关闭后进程才退出（默认 1 = 流关闭即进程已退出）。
	// 断流场景用 2：第一轮流关闭时进程仍在运行（真截断），重挂拿到尾部后才退出。
	exitAfterAttach int
	attachClosed    int
	exitCode        int
	attachFailAfter int // >0 时第 N 次之后的 attach 返回 500
	attachCalls     int
	// captureLog：容器内 tee 落盘的完整日志；兜底 exec 按 tail -c +N 截取后返回
	captureLog string
	cmds       map[string][]string // exec id → Cmd（断言命令包壳）
}

func (f *fakeDaemon) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/containers/fake-container/exec"):
		f.handleExecCreate(w, r)
	case strings.HasSuffix(r.URL.Path, "/start"):
		f.handleAttach(w, execIDFromPath(r.URL.Path))
	case strings.HasSuffix(r.URL.Path, "/json"):
		f.handleInspect(w)
	default:
		http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
	}
}

// execIDFromPath 从 /v1.47/exec/{id}/start 里取 exec id
func execIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if p == "exec" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// lastCmd 返回某个 exec 实际执行的命令（断言包壳/补齐命令用）
func (f *fakeDaemon) lastCmd(id string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cmds[id]
}

// handleExecCreate 模拟 POST /containers/{id}/exec → {Id}
// 兜底命令（tee 日志的 tail/rm）用独立 exec id，避免干扰命令 exec 的 inspect 序列。
func (f *fakeDaemon) handleExecCreate(w http.ResponseWriter, r *http.Request) {
	var body struct{ Cmd []string }
	_ = json.NewDecoder(r.Body).Decode(&body)
	id := "fake-exec"
	for _, arg := range body.Cmd {
		if strings.Contains(arg, "tail -c +") || strings.Contains(arg, ".flowx-capture-") && strings.Contains(arg, "rm -f") {
			id = "capture-exec"
			break
		}
	}
	f.mu.Lock()
	if f.cmds == nil {
		f.cmds = map[string][]string{}
	}
	f.cmds[id] = body.Cmd
	f.mu.Unlock()

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"Id": id})
}

func (f *fakeDaemon) handleAttach(w http.ResponseWriter, execID string) {
	f.mu.Lock()
	f.attachCalls++
	call := f.attachCalls
	fail := f.attachFailAfter > 0 && call > f.attachFailAfter
	var lines []string
	if execID == "capture-exec" {
		f.attachCalls-- // 兜底 exec 不参与命令 exec 的 attach 计数
		// 忠实模拟 `tail -c +N <log>`：从第 N 字节（1 起）返回，这样"跳过已送达前缀"
		// 这件事本身也被验证（假实现若整段返回，重复投递的断言就会失败）。
		if offset := parseTailOffset(f.cmds["capture-exec"]); offset > 0 && offset-1 < len(f.captureLog) {
			lines = []string{f.captureLog[offset-1:]}
		}
	} else if f.roundIdx < len(f.rounds) {
		lines = f.rounds[f.roundIdx]
		f.roundIdx++
	}
	f.mu.Unlock()

	if fail {
		http.Error(w, "attach refused", http.StatusInternalServerError)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack unsupported", http.StatusInternalServerError)
		return
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return
	}
	// 只有真正建流的 attach 才算"流已关闭"（失败的重挂不计），
	// 这样才能用 exitAfterAttach 准确建模"第 N 轮流关闭时进程是否已退出"。
	defer func() {
		_ = conn.Close()
		f.mu.Lock()
		f.attachClosed++
		f.mu.Unlock()
	}()

	rw.WriteString("HTTP/1.1 101 UPGRADED\r\n" +
		"Content-Type: application/vnd.docker.raw-stream\r\n" +
		"Connection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
	for _, line := range lines {
		if strings.HasSuffix(line, "\n") {
			writeStdoutFrame(rw, line)
			continue
		}
		writeStdoutFrame(rw, line+"\n")
	}
	_ = rw.Flush()
	// 关闭连接模拟"attach 流结束"：可能是命令真的跑完，也可能是流被中途掐断。
	// 先半关（FIN）再断开，避免未读数据导致 RST（会让客户端读到 connection reset，
	// 属真实世界另一种断流形态，但会让本用例的"正常结束"语义变得不确定）。
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
		time.Sleep(10 * time.Millisecond)
	}
}

func (f *fakeDaemon) handleInspect(w http.ResponseWriter) {
	f.mu.Lock()
	var running bool
	if f.exitAfterAttach == 0 {
		f.exitAfterAttach = 1
	}
	running = f.attachClosed < f.exitAfterAttach
	if len(f.inspectSequence) > 0 {
		if f.inspectIdx < len(f.inspectSequence) {
			running = f.inspectSequence[f.inspectIdx]
		} else {
			running = f.inspectSequence[len(f.inspectSequence)-1]
		}
	}
	f.inspectIdx++
	code := f.exitCode
	f.mu.Unlock()

	_ = json.NewEncoder(w).Encode(map[string]any{
		"ID":         "fake-exec",
		"Running":    running,
		"ExitCode":   code,
		"Pid":        1,
		"ExitCodeCh": nil,
	})
}

// parseTailOffset 从 `tail -c +N <log>` 命令里取出 N（1 起；无 tail 则 0）
func parseTailOffset(cmd []string) int {
	for _, arg := range cmd {
		i := strings.Index(arg, "tail -c +")
		if i < 0 {
			continue
		}
		rest := strings.Fields(arg[i+len("tail -c +"):])
		if len(rest) == 0 {
			continue
		}
		if n, err := strconv.Atoi(rest[0]); err == nil {
			return n
		}
	}
	return 0
}

func writeStdoutFrame(w io.Writer, s string) {
	hdr := make([]byte, 8)
	hdr[0] = 1 // stdout
	binary.BigEndian.PutUint32(hdr[4:], uint32(len(s)))
	_, _ = w.Write(hdr)
	_, _ = io.WriteString(w, s)
}

func newFakeExecutor(t *testing.T, d *fakeDaemon) *DockerExecutor {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(d.serveHTTP))
	t.Cleanup(srv.Close)

	// docker 客户端的 hijack 路径只认 tcp:// 前缀（默认 dialer 用 proto 当 network）
	cli, err := client.NewClientWithOpts(
		client.WithHost("tcp://"+strings.TrimPrefix(srv.URL, "http://")),
		client.WithVersion("1.47"))
	if err != nil {
		t.Fatalf("new fake docker client: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	exec := NewDockerExecutorWithClient(cli)
	exec.containerID = "fake-container" // 跳过 Prepare，直接进入 exec 流程
	return exec
}

func collectOutput(ch chan any, done <-chan struct{}) func() string {
	var sb strings.Builder
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case v, ok := <-ch:
				if !ok {
					return
				}
				switch data := v.(type) {
				case []byte:
					sb.Write(data)
				case string:
					sb.WriteString(data)
				}
			case <-done:
				return
			}
		}
	}()
	return func() string {
		time.Sleep(50 * time.Millisecond)
		wg.Wait()
		return sb.String()
	}
}

// 断流 + 重挂续读：第一轮输出后连接被掐断（进程仍在运行），重挂后读到尾部结果块。
func TestExecStream_TruncatedThenReattachRecovers(t *testing.T) {
	d := &fakeDaemon{
		rounds: [][]string{
			{"[ksampler] job submitted", "[job] running 1/2"},
			{"[job] running 2/2", "```flowx-yaml", "latent: \"abc\"", "```"},
		},
		exitAfterAttach: 2, // 第一轮 attach 关闭时进程仍在运行（真截断），重挂后才退出
	}
	exec := newFakeExecutor(t, d)

	out := make(chan any, 128)
	closed := make(chan struct{})
	getOut := collectOutput(out, closed)
	appendCb := func(b []byte) { out <- b }

	truncated, err := exec.executeCommandInContainerStreaming(context.Background(),
		"python main.py", nil, "", appendCb, nil, nil)
	close(closed)

	if err != nil {
		t.Fatalf("expected recovery without error, got %v", err)
	}
	if truncated != 1 {
		t.Fatalf("truncated = %d, want 1", truncated)
	}
	got := getOut()
	for _, want := range []string{"[job] running 2/2", "```flowx-yaml", "latent: \"abc\"", "stream truncated, re-attach 1/3"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n---\n%s", want, got)
		}
	}
}

// 重挂失败：必须报错（绝不静默当成功）
func TestExecStream_TruncatedReattachFails(t *testing.T) {
	d := &fakeDaemon{
		rounds:          [][]string{{"partial output"}},
		exitAfterAttach: 2,
		attachFailAfter: 1,
	}
	exec := newFakeExecutor(t, d)

	truncated, err := exec.executeCommandInContainerStreaming(context.Background(),
		"python main.py", nil, "", func([]byte) {}, nil, nil)

	if err == nil {
		t.Fatal("expected error when re-attach fails, got nil (silent success is the bug)")
	}
	if truncated != 1 {
		t.Fatalf("truncated = %d, want 1 attempt counted", truncated)
	}
	if !strings.Contains(err.Error(), "re-attach failed") {
		t.Fatalf("error should mention re-attach failure, got: %v", err)
	}
}

// 正常结束（流关闭且进程已退出）：不算截断
func TestExecStream_NormalEnd(t *testing.T) {
	d := &fakeDaemon{
		rounds:          [][]string{{"```flowx-yaml", "seed: \"1\"", "```"}},
		inspectSequence: []bool{false},
	}
	exec := newFakeExecutor(t, d)

	var sb strings.Builder
	truncated, err := exec.executeCommandInContainerStreaming(context.Background(),
		"python main.py", nil, "", func(b []byte) { sb.Write(b) }, nil, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if truncated != 0 {
		t.Fatalf("truncated = %d, want 0", truncated)
	}
	if !strings.Contains(sb.String(), "seed: \"1\"") {
		t.Fatalf("output not captured: %q", sb.String())
	}
}

// 退出码非 0 仍要失败（回归守卫）
func TestExecStream_NonZeroExit(t *testing.T) {
	d := &fakeDaemon{
		rounds:          [][]string{{"boom\n"}},
		inspectSequence: []bool{false},
		exitCode:        2,
	}
	exec := newFakeExecutor(t, d)

	_, err := exec.executeCommandInContainerStreaming(context.Background(),
		"false", nil, "", func([]byte) {}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "exited with code 2") {
		t.Fatalf("expected exit code error, got: %v", err)
	}
}

// StepResult.StreamTruncated 透传（宿主据此提示"输出可能不完整"）
func TestExecuteCommandStreaming_SetsStreamTruncated(t *testing.T) {
	d := &fakeDaemon{
		rounds: [][]string{
			{"first"},
			{"second"},
		},
		exitAfterAttach: 2,
	}
	exec := newFakeExecutor(t, d)

	resultChan := make(chan any, 64)
	commandChan := make(chan any, 1)
	inputChan := make(chan []byte)
	commandChan <- executor.CommandWrapper{StepName: "run", Command: "python main.py"}
	close(commandChan)

	go exec.Transfer(context.Background(), resultChan, commandChan, inputChan)

	deadline := time.After(10 * time.Second)
	for {
		select {
		case v := <-resultChan:
			sr, ok := v.(*executor.StepResult)
			if !ok {
				continue
			}
			if sr.Error != nil {
				t.Fatalf("step failed: %v", sr.Error)
			}
			if !sr.StreamTruncated {
				t.Fatal("StreamTruncated should be true after a truncated stream")
			}
			return
		case <-deadline:
			t.Fatal("timed out waiting for StepResult")
		}
	}
}

// 帧头与行内容不会被混淆（非 TTY 模式 stdcopy 解复用回归）
func TestExecStream_StdCopyFraming(t *testing.T) {
	d := &fakeDaemon{
		rounds:          [][]string{{"line-a", "line-b"}},
		inspectSequence: []bool{false},
	}
	exec := newFakeExecutor(t, d)

	var lines []string
	var mu sync.Mutex
	_, err := exec.executeCommandInContainerStreaming(context.Background(), "cmd", nil, "",
		func(b []byte) { mu.Lock(); lines = append(lines, strings.TrimRight(string(b), "\n")); mu.Unlock() },
		nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(lines) < 2 || lines[0] != "line-a" || lines[1] != "line-b" {
		t.Fatalf("lines = %v, want [line-a line-b ...]", lines)
	}
}

// settle 窗口：进程刚退出时 docker 可能瞬时仍报 Running=true，不能据此判定断流
func TestExecRunningSettled(t *testing.T) {
	// 序列：true（瞬时假 running）→ false（真实状态）⇒ 应判为已退出
	d1 := &fakeDaemon{inspectSequence: []bool{true, false}}
	exec1 := newFakeExecutor(t, d1)
	running, err := exec1.execRunningSettled(context.Background(), "fake-exec", 500*time.Millisecond)
	if err != nil || running {
		t.Fatalf("settled(seq true,false) = (%v,%v), want (false,nil)", running, err)
	}

	// 序列恒 true ⇒ 窗口内持续 running，判定截断
	d2 := &fakeDaemon{inspectSequence: []bool{true}}
	exec2 := newFakeExecutor(t, d2)
	start := time.Now()
	running, err = exec2.execRunningSettled(context.Background(), "fake-exec", 400*time.Millisecond)
	if err != nil || !running {
		t.Fatalf("settled(seq true) = (%v,%v), want (true,nil)", running, err)
	}
	if elapsed := time.Since(start); elapsed < 300*time.Millisecond {
		t.Fatalf("settle returned too early: %v", elapsed)
	}
}

// ---------- 容器内 tee 兜底（captureTag 非空，仅声明 extract 的步骤） ----------

// 未声明 extract：命令原样下发，不加包壳
func TestCapture_NoTagNoWrap(t *testing.T) {
	d := &fakeDaemon{rounds: [][]string{{"plain output"}}}
	exec := newFakeExecutor(t, d)

	_, err := exec.executeCommandInContainerStreaming(context.Background(),
		"python main.py", nil, "", func([]byte) {}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmd := strings.Join(d.lastCmd("fake-exec"), " ")
	if strings.Contains(cmd, "tee ") || strings.Contains(cmd, ".flowx-capture-") {
		t.Fatalf("命令不应被包壳: %q", cmd)
	}
}

// 声明 extract：命令被包成 tee 落盘 + 退出码走 rc 文件
func TestCapture_TagWrapsCommand(t *testing.T) {
	d := &fakeDaemon{rounds: [][]string{{"```flowx-yaml", "latent: abc", "```"}}}
	exec := newFakeExecutor(t, d)

	_, err := exec.executeCommandInContainerStreaming(context.Background(),
		"python main.py", nil, "unit-wrap", func([]byte) {}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmd := strings.Join(d.lastCmd("fake-exec"), " ")
	for _, want := range []string{
		"tee /tmp/.flowx-capture-unit-wrap.log",
		"echo $? > /tmp/.flowx-capture-unit-wrap.rc",
		"exit ${__flowx_rc:-1}",
		"python main.py", // 原命令仍在
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("包壳命令缺少 %q: %q", want, cmd)
		}
	}
	// 流未中断：只做清理，不补齐（tail 不应出现）
	if clean := strings.Join(d.lastCmd("capture-exec"), " "); strings.Contains(clean, "tail -c +") {
		t.Fatalf("未断流不应补齐: %q", clean)
	} else if !strings.Contains(clean, "rm -f /tmp/.flowx-capture-unit-wrap.log") {
		t.Fatalf("未断流应清理临时文件: %q", clean)
	}
}

// 断流 + 重挂接回但中间缺了尾部：从容器内日志按"已送达字节数"补齐，且不重复投递
func TestCapture_ReplaysMissingTailAfterTruncation(t *testing.T) {
	full := "progress 1/2\n```flowx-yaml\nlatent: abc\n```\n"
	d := &fakeDaemon{
		rounds:          [][]string{{"progress 1/2"}, {}}, // 重挂没拿到任何字节
		exitAfterAttach: 2,                                // 第一轮流关闭时进程仍在运行 → 真截断
		captureLog:      full,                             // 容器内 tee 日志（完整输出）
	}
	exec := newFakeExecutor(t, d)

	var sb strings.Builder
	truncated, err := exec.executeCommandInContainerStreaming(context.Background(),
		"python main.py", nil, "unit-replay", func(b []byte) { sb.Write(b) }, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if truncated != 1 {
		t.Fatalf("truncated = %d, want 1", truncated)
	}
	got := sb.String()
	if !strings.Contains(got, "latent: abc") {
		t.Fatalf("补齐输出未送达: %q", got)
	}
	if n := strings.Count(got, "progress 1/2"); n != 1 {
		t.Fatalf("已送达前缀被重复投递 %d 次: %q", n, got)
	}
	replay := strings.Join(d.lastCmd("capture-exec"), " ")
	// streamBytes = len("progress 1/2\n") = 13 → tail 从第 14 字节开始
	if !strings.Contains(replay, "tail -c +14 /tmp/.flowx-capture-unit-replay.log") {
		t.Fatalf("补齐命令偏移不对: %q", replay)
	}
	if !strings.Contains(replay, "rm -f /tmp/.flowx-capture-unit-replay.log") {
		t.Fatalf("补齐后应清理临时文件: %q", replay)
	}
}

// 重挂用尽（直接失败）：仍尽量补齐尾部，失败现场要完整
func TestCapture_ReplaysOnGiveUp(t *testing.T) {
	d := &fakeDaemon{
		rounds:          [][]string{{"progress 1/2"}},
		exitAfterAttach: 99, // 进程永不退出 → 重挂 3 次后放弃
		captureLog:      "progress 1/2\n```flowx-yaml\nlatent: abc\n```\n",
	}
	exec := newFakeExecutor(t, d)

	var sb strings.Builder
	truncated, err := exec.executeCommandInContainerStreaming(context.Background(),
		"python main.py", nil, "unit-giveup", func(b []byte) { sb.Write(b) }, nil, nil)
	if err == nil {
		t.Fatal("流持续中断必须失败（不得静默当成功）")
	}
	if truncated != execStreamMaxReattach {
		t.Fatalf("truncated = %d, want %d", truncated, execStreamMaxReattach)
	}
	if !strings.Contains(sb.String(), "latent: abc") {
		t.Fatalf("失败前应补齐尾部: %q", sb.String())
	}
}

// 多行命令：内层组整体重定向，保证每行 stderr 都进日志（否则日志比流短，
// "日志前缀=已送达"的补齐前提被破坏）
func TestCapture_MultilineCommandRedirection(t *testing.T) {
	d := &fakeDaemon{rounds: [][]string{{"ok"}}}
	exec := newFakeExecutor(t, d)

	_, err := exec.executeCommandInContainerStreaming(context.Background(),
		"echo a\npython b.py", nil, "unit-multi", func([]byte) {}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmd := strings.Join(d.lastCmd("fake-exec"), " ")
	if !strings.Contains(cmd, "{ { echo a\npython b.py\n} 2>&1;") {
		t.Fatalf("多行命令未整体重定向: %q", cmd)
	}
}
