package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/LerkoX/flowx/executor"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
	"gopkg.in/yaml.v2"
)

// defaultDaemonTimeout docker daemon 控制面请求的默认响应超时。
//
// 动机（exec 407 事故）：host 的 TCP 端口能被连上但 daemon 不响应（隧道断开后
// 中间设备仍接受连接）时，docker client 默认只设了 10s 拨号超时、没有响应头超时，
// 控制面请求会永久挂起 —— 节点永远停在 running，整条流水线卡死且没有任何失败信号。
// 这里给控制面请求一个上限，让这类异常快速失败（见 controlContext / attachExec）。
const defaultDaemonTimeout = 15 * time.Second

// DockerExecutor Docker执行器实现
type DockerExecutor struct {
	client            *client.Client
	containerID       string
	image             string
	workdir           string
	env               map[string]string
	volumes           map[string]string
	network           string
	registry          string
	host              string             // daemon 地址（tcp://… / ssh://… / unix://…），空表示从环境变量读取（DOCKER_HOST 等）
	tlsVerify         bool               // 是否启用 TLS 校验
	certPath          string             // TLS 证书目录（含 ca.pem/cert.pem/key.pem），默认为 ~/.docker
	tty               bool               // 是否启用 TTY 模式
	ttyHeight         uint               // TTY 终端高度
	ttyWidth          uint               // TTY 终端宽度
	daemonTimeout     time.Duration      // daemon 控制面请求响应超时（0 表示 defaultDaemonTimeout）
	currentExecCancel context.CancelFunc // 用于取消当前执行的命令
	mu                sync.RWMutex
}

// parseInputRequest 解析输入请求代码块内容
// 支持 YAML 或 JSON 格式
func parseInputRequest(content string) *executor.InputRequest {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	var req executor.InputRequest

	if err := yaml.Unmarshal([]byte(content), &req); err == nil && req.Type != "" {
		return &req
	}

	if err := json.Unmarshal([]byte(content), &req); err == nil && req.Type != "" {
		return &req
	}

	return nil
}

// NewDockerExecutor 创建新的Docker执行器
//
// client 不在此处创建，而是在 Prepare 时惰性创建（ensureClient）：
// 此时 adapter 配置（host/tlsVerify/certPath 等）已应用完毕，
// 才能决定连接哪个 daemon。未配置 host 时回退到环境变量（FromEnv），
// 与历史行为一致。
func NewDockerExecutor() (*DockerExecutor, error) {
	return &DockerExecutor{
		env:     make(map[string]string),
		volumes: make(map[string]string),
	}, nil
}

// ensureClient 惰性创建 Docker client（调用方须持有 d.mu）。
// 配置了 host 时按 host/tlsVerify/certPath 构造；否则读取进程环境变量
// （DOCKER_HOST / DOCKER_TLS_VERIFY / DOCKER_CERT_PATH / DOCKER_API_VERSION）。
func (d *DockerExecutor) ensureClient() error {
	if d.client != nil {
		return nil
	}

	opts := []client.Opt{client.WithAPIVersionNegotiation()}
	if d.host != "" {
		opts = append(opts, client.WithHost(d.host))
		if d.tlsVerify || d.certPath != "" {
			certDir := d.certPath
			if certDir == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("tlsVerify requires certPath (failed to locate home dir): %w", err)
				}
				certDir = filepath.Join(home, ".docker")
			}
			opts = append(opts, client.WithTLSClientConfig(
				filepath.Join(certDir, "ca.pem"),
				filepath.Join(certDir, "cert.pem"),
				filepath.Join(certDir, "key.pem"),
			))
		}
	} else {
		opts = append([]client.Opt{client.FromEnv}, opts...)
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return fmt.Errorf("failed to create docker client (host=%q): %w", d.host, err)
	}
	d.client = cli
	return nil
}

// daemonResponseTimeout 返回 daemon 控制面请求响应超时（未配置时用默认值）
func (d *DockerExecutor) daemonResponseTimeout() time.Duration {
	if d.daemonTimeout > 0 {
		return d.daemonTimeout
	}
	return defaultDaemonTimeout
}

// controlContext 为单次控制面调用派生带超时的 context。
// 调用方 ctx 已有不晚于本超时的 deadline 时原样返回（尊重更紧的上层约束）。
func (d *DockerExecutor) controlContext(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := d.daemonResponseTimeout()
	if dl, ok := ctx.Deadline(); ok && time.Until(dl) <= timeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

// daemonErrHint 对超时/连接类错误补充 host 与排查提示，
// 把"daemon 不可达"与"镜像/容器不存在"两类失败在日志里区分开。
func (d *DockerExecutor) daemonErrHint(err error) error {
	if err == nil {
		return nil
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) {
		return fmt.Errorf(
			"docker daemon not responding (host=%q, timeout=%s): %w; "+
				"check the executor host/tlsVerify/certPath and that the daemon is reachable",
			d.host, d.daemonResponseTimeout(), err)
	}
	return err
}

// NewDockerExecutorWithClient 使用指定的Docker客户端创建执行器
func NewDockerExecutorWithClient(cli *client.Client) *DockerExecutor {
	return &DockerExecutor{
		client:  cli,
		env:     make(map[string]string),
		volumes: make(map[string]string),
	}
}

// Prepare 准备Docker环境
// 1. 检查/拉取镜像
// 2. 创建并启动容器
func (d *DockerExecutor) Prepare(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 惰性创建 client（此时 host/tlsVerify/certPath 等配置已应用）
	if err := d.ensureClient(); err != nil {
		return err
	}

	// 如果没有指定镜像，使用默认镜像
	if d.image == "" {
		d.image = "alpine:latest"
	}

	// 解析镜像名称（处理registry）
	fullImage := d.resolveImageName()

	// 检查镜像是否存在，不存在则拉取
	if err := d.pullImageIfNeeded(ctx, fullImage); err != nil {
		return err
	}

	// 构建容器配置
	// 当启用 TTY 时，容器本身也需要开启 TTY，以保证 exec attach 的 TTY 模式能正常工作
	containerConfig := &container.Config{
		Image:        fullImage,
		Cmd:          []string{"sleep", "3600"},
		WorkingDir:   d.workdir,
		Env:          d.buildEnvList(),
		AttachStdout: true,
		AttachStderr: true,
		Tty:          d.tty,
		OpenStdin:    d.tty,
	}

	// 构建主机配置
	hostConfig := &container.HostConfig{
		Mounts:     d.buildMounts(),
		AutoRemove: false,
	}

	// 设置网络模式
	if d.network != "" {
		hostConfig.NetworkMode = container.NetworkMode(d.network)
	}

	// 创建/启动/等待容器属于控制面握手：daemon 无响应时必须有界失败，
	// 否则节点会永远停在 running（exec 407 事故）
	controlCtx, cancel := d.controlContext(ctx)
	defer cancel()

	// 创建容器
	resp, err := d.client.ContainerCreate(controlCtx, containerConfig, hostConfig, nil, nil, fmt.Sprintf("flowx-%d", time.Now().UnixNano()))
	if err != nil {
		return fmt.Errorf("failed to create container: %w", d.daemonErrHint(err))
	}

	d.containerID = resp.ID

	// 启动容器
	if err := d.client.ContainerStart(controlCtx, d.containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", d.daemonErrHint(err))
	}

	// 等待容器启动完成
	if err := d.waitForContainer(controlCtx); err != nil {
		return fmt.Errorf("container failed to start: %w", err)
	}

	return nil
}

// Destruction 销毁Docker环境
// 停止并删除容器
func (d *DockerExecutor) Destruction(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.containerID == "" {
		return nil
	}

	// 停止容器
	timeout := 10
	_ = d.client.ContainerStop(ctx, d.containerID, container.StopOptions{
		Timeout: &timeout,
	})

	// 删除容器
	if err := d.client.ContainerRemove(ctx, d.containerID, container.RemoveOptions{
		Force: true,
	}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	d.containerID = ""
	return nil
}

// Transfer 在Docker容器中执行命令
// in 接收执行数据（包括步骤信息），out 发送执行结果
// inputChan 用于接收交互式输入数据，可为 nil（不需要输入时）
//
// 当 ctx 被取消时，会立即停止执行新命令，并终止当前正在容器内执行的命令
func (d *DockerExecutor) Transfer(ctx context.Context, resultChan chan<- any, commandChan <-chan any, inputChan <-chan []byte) {
	// 创建一个可取消的内部上下文，用于控制当前命令的执行
	// execCtx 是 ctx 的子上下文，外部取消会自动传播，无需额外监听。
	// commandChan 关闭时下方 for 循环的 !ok 分支会直接退出。
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		// 检查上下文是否已取消
		select {
		case <-execCtx.Done():
			return
		case data, ok := <-commandChan:
			if !ok {
				return
			}

			// 处理 commandWrapper 类型
			cmdWrapper, ok := data.(executor.CommandWrapper)
			if !ok {
				safeSend(resultChan, fmt.Errorf("unsupported data type: %T, expected CommandWrapper", data))
				continue
			}
			// 执行命令（携带步骤名称；CaptureOutput 步骤启用容器内 tee 兜底）
			d.executeCommandStreaming(execCtx, cmdWrapper.Command, cmdWrapper.StepName, cmdWrapper.Env, cmdWrapper.CaptureOutput, resultChan, inputChan)
		}
	}
}

// executeCommandStreaming 执行命令并实时流式输出
func (d *DockerExecutor) executeCommandStreaming(ctx context.Context, command string, stepName string, env map[string]string, captureOutput bool, resultChan chan<- any, inputChan <-chan []byte) {
	startTime := time.Now()

	inputRequestChan := make(chan *executor.InputRequest, 1)
	onInputRequest := func(req *executor.InputRequest) {
		select {
		case inputRequestChan <- req:
		default:
		}
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case req := <-inputRequestChan:
				if req != nil {
					safeSend(resultChan, &executor.InputRequestEvent{
						StepName: stepName,
						Request:  req,
					})
				}
			}
		}
	}()

	// 声明了 extract 的步骤才启用容器内 tee 兜底（短任务节点无需多花一次 exec 清理）
	capt := ""
	if captureOutput {
		capt = newCaptureTag(stepName)
	}

	truncated, err := d.executeCommandInContainerStreaming(ctx, command, env, capt, func(data []byte) {
		safeSend(resultChan, data)
	}, inputChan, onInputRequest)

	// 发送最终结果
	safeSend(resultChan, &executor.StepResult{
		StepName:        stepName,
		Command:         command,
		Output:          "",
		Error:           err,
		StartTime:       startTime,
		FinishTime:      time.Now(),
		StreamTruncated: truncated > 0,
	})
}

// executeCommandInContainerStreaming 在容器中执行命令并实时流式输出。
//
// captureTag 非空时（仅节点声明了 extract 的步骤）在容器内加 tee 兜底：
// 完整输出同时写 /tmp/.flowx-capture-<tag>.log，退出码写 .rc 并由 wrapper 显式 exit，
// 这样“输出落盘”与“退出码语义”都不依赖 attach 流是否完整（见下方 replayCapturedTail）。
//
// 返回值 truncated 表示 attach 读取流曾被中断（已尽力重挂续读，次数=truncated）；
// 重挂仍无法续接时返回错误——**绝不静默当成功**：日志流被截断会让节点输出块丢失，
// 下游节点随后报"缺参"，把排查方向带偏（exec 364 事故：cpolar 上的 docker exec
// 流被截断 → KSampler 绿灯但无输出 → VAEDecode 报 missing required parameter）。
func (d *DockerExecutor) executeCommandInContainerStreaming(ctx context.Context, command string, env map[string]string, captureTag string, outputCallback func([]byte), inputChan <-chan []byte, onInputRequest func(*executor.InputRequest)) (int, error) {
	d.mu.RLock()
	containerID := d.containerID
	d.mu.RUnlock()

	if containerID == "" {
		return 0, fmt.Errorf("container not prepared")
	}

	shell := d.detectShell()

	// 声明了 extract 的步骤：命令包壳，完整输出经 tee 落盘（容器内 /tmp）。
	// 包壳只改变重定向与退出码来源，不影响实时性（tee 边读边写）。
	if captureTag != "" {
		command = wrapCommandForCapture(command, captureTag)
	}

	execConfig := container.ExecOptions{
		Cmd:          []string{shell, "-c", command},
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  inputChan != nil,
		Tty:          d.tty,
	}
	// 命令级环境变量（dag 渲染终值）经 exec 配置真实注入，不经 shell 解析
	if len(env) > 0 {
		envList := make([]string, 0, len(env))
		for k, v := range env {
			envList = append(envList, k+"="+v)
		}
		execConfig.Env = envList
	}

	// 控制面调用（创建 exec）：daemon 无响应时有界失败，避免节点卡在 running
	createCtx, createCancel := d.controlContext(ctx)
	execResp, err := d.client.ContainerExecCreate(createCtx, containerID, execConfig)
	createCancel()
	if err != nil {
		return 0, fmt.Errorf("failed to create exec: %w", err)
	}

	attachResp, err := d.attachExec(ctx, execResp.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer func() {
		// 闭包捕获变量：重挂后 attachResp 已换为新连接，关闭的必须是当前这条
		attachResp.Close()
	}()

	// 断流重挂时会替换连接：取消信号与 stdin 写入都必须指向当前连接，
	// 故统一经 curConn 读写（attachResp 本身在重挂时被替换）。
	var connMu sync.Mutex
	curConn := attachResp.Conn

	// 如果启用 TTY，应用终端尺寸
	if d.tty && (d.ttyWidth > 0 || d.ttyHeight > 0) {
		resizeCtx, resizeCancel := d.controlContext(ctx)
		_ = d.client.ContainerExecResize(resizeCtx, execResp.ID, container.ResizeOptions{
			Width:  d.ttyWidth,
			Height: d.ttyHeight,
		})
		resizeCancel()
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	execCtx, execCancel := context.WithCancel(ctx)
	defer execCancel()

	// 使用 sync.Once 保证取消逻辑只执行一次，避免重复关闭连接或重复取消
	var cancelOnce sync.Once
	d.mu.Lock()
	d.currentExecCancel = func() {
		cancelOnce.Do(func() {
			connMu.Lock()
			conn := curConn
			connMu.Unlock()
			if conn != nil {
				_, _ = conn.Write([]byte{0x03})
			}
			execCancel()
		})
	}
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		d.currentExecCancel = nil
		d.mu.Unlock()
	}()

	go func() {
		<-ctx.Done()
		d.mu.RLock()
		cancel := d.currentExecCancel
		d.mu.RUnlock()
		if cancel != nil {
			cancel()
		}
	}()

	if inputChan != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-execCtx.Done():
					return
				case <-done:
					return
				case data, ok := <-inputChan:
					if !ok {
						return
					}
					if len(data) > 0 {
						connMu.Lock()
						conn := curConn
						connMu.Unlock()
						if conn != nil {
							_, _ = conn.Write(data)
						}
					}
				}
			}
		}()
	}

	// 读取一轮 attach 流至结束。非 TTY 模式下 docker attach 是带 8 字节帧头的
	// 多路复用流（stdout/stderr 合帧），直接 scan 会把帧头混进日志行（污染行首
	// 标记解析，如 FLOWX_PREVIEW 拦截）→ 经 stdcopy 解复用到单一管道后再扫描；
	// TTY 模式本身即裸流。读取状态（flowx-input 代码块解析）跨重挂轮次保留。
	var buffer strings.Builder
	inInputBlock := false

	// 已从 attach 流读到的字节数 ≈ 容器内日志的前缀长度：断流兜底时据此跳过
	// 已送达的前缀，只补发缺失的尾部（避免整段重复）。重挂提示行不计入（它不在日志里）。
	streamBytes := 0

	scanRound := func(reader io.Reader) error {
		var outputReader io.Reader = reader
		if !d.tty {
			pr, pw := io.Pipe()
			go func() {
				_, err := stdcopy.StdCopy(pw, pw, reader)
				_ = pw.CloseWithError(err)
			}()
			outputReader = pr
		}

		scanner := bufio.NewScanner(outputReader)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			streamBytes += len(line) + 1 // ≈ 该行在容器内日志中占用的字节数

			if strings.TrimSpace(line) == "```flowx-input" {
				inInputBlock = true
				buffer.Reset()
				continue
			}

			if inInputBlock && strings.TrimSpace(line) == "```" {
				inInputBlock = false
				if req := parseInputRequest(buffer.String()); req != nil && onInputRequest != nil {
					onInputRequest(req)
				}
				continue
			}

			if inInputBlock {
				buffer.WriteString(line)
				buffer.WriteString("\n")
				continue
			}

			if outputCallback != nil {
				outputCallback(append([]byte(line), '\n'))
			}
		}
		return scanner.Err()
	}

	truncated := 0
	for {
		scanErr := scanRound(attachResp.Reader)

		// 流结束：仅凭 EOF 无法区分"命令真的退出"与"流被中途掐断"
		// （两者都是干净关闭），但进程是否仍在运行可以区分：
		// 命令正常结束时进程必然已退出。settle 窗口避开"刚退出仍报 running"的竞态。
		running, inspectErr := d.execRunningSettled(ctx, execResp.ID, execStreamSettleWindow)
		if inspectErr != nil {
			close(done)
			wg.Wait()
			return truncated, fmt.Errorf("failed to inspect exec: %w", inspectErr)
		}

		switch classifyStreamEnd(running, truncated, execStreamMaxReattach) {
		case streamOutcomeDone:
			if scanErr != nil && scanErr != io.EOF && outputCallback != nil {
				outputCallback([]byte(fmt.Sprintf("\n[stream error: %v]\n", scanErr)))
			}
		case streamOutcomeGiveUp:
			close(done)
			wg.Wait()
			// 已确定失败，但仍把容器内的尾部补发出去：失败现场越完整越好排查
			d.replayCapturedTail(ctx, captureTag, streamBytes, outputCallback)
			return truncated, fmt.Errorf(
				"docker exec output stream truncated while command still running "+
					"(re-attached %d times, last read error: %v)", truncated, scanErr)
		case streamOutcomeReattach:
			truncated++
			if outputCallback != nil {
				outputCallback([]byte(fmt.Sprintf(
					"[flowx] docker exec stream truncated, re-attach %d/%d ...\n",
					truncated, execStreamMaxReattach)))
			}
			time.Sleep(execStreamReattachBackoff(truncated))

			newResp, aerr := d.attachExec(ctx, execResp.ID)
			if aerr != nil {
				close(done)
				wg.Wait()
				d.replayCapturedTail(ctx, captureTag, streamBytes, outputCallback)
				return truncated, fmt.Errorf(
					"docker exec stream truncated and re-attach failed: %w", aerr)
			}
			connMu.Lock()
			old := attachResp
			attachResp = newResp
			curConn = newResp.Conn
			connMu.Unlock()
			old.Close()
			continue
		}
		break
	}

	close(done)
	wg.Wait()

	// 流曾中断（已重挂接回）：实时流中间可能缺了一段（重挂前未及读到的字节），
	// 用容器内日志补齐尾部；未中断则只清理临时文件（容器按 executor 复用，
	// 不清理会在硬盘上累加，故不省这一次 exec）。
	if captureTag != "" {
		if truncated > 0 {
			d.replayCapturedTail(ctx, captureTag, streamBytes, outputCallback)
		} else {
			d.cleanupCapture(ctx, captureTag)
		}
	}

	inspectResp, err := d.execInspect(ctx, execResp.ID)
	if err != nil {
		return truncated, fmt.Errorf("failed to inspect exec: %w", err)
	}
	if inspectResp.ExitCode != 0 {
		return truncated, fmt.Errorf("command exited with code %d", inspectResp.ExitCode)
	}

	return truncated, nil
}

// execStreamMaxReattach docker exec 输出流被中断后的最大重挂次数
// （退避 0.5s/1s/2s，每次重挂能继续读到之后的输出）。
const execStreamMaxReattach = 3

// attachResultContainerExecAttach 结果（供带超时的 attachExec 与延迟回收使用）
type attachResult struct {
	resp types.HijackedResponse
	err  error
}

// attachExec 带超时的 exec attach。
//
// docker 的 attach 走裸连接（client.hijack → http.ReadResponse），不受 HTTP
// transport 的响应头超时约束：daemon 接受连接但不回 upgrade 响应时会永久阻塞。
// 这里用独立计时兜底，超时即让节点失败；attach 拿到连接之后的流式读取（可能
// 持续很久）不受影响。
func (d *DockerExecutor) attachExec(ctx context.Context, execID string) (types.HijackedResponse, error) {
	timeout := d.daemonResponseTimeout()
	ch := make(chan attachResult, 1)
	go func() {
		resp, err := d.client.ContainerExecAttach(ctx, execID, container.ExecAttachOptions{Tty: d.tty})
		ch <- attachResult{resp: resp, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case r := <-ch:
		return r.resp, r.err
	case <-ctx.Done():
		go closeLateAttach(ch)
		return types.HijackedResponse{}, ctx.Err()
	case <-timer.C:
		go closeLateAttach(ch)
		return types.HijackedResponse{}, fmt.Errorf(
			"docker exec attach timed out after %s (host=%q, daemon not responding)", timeout, d.host)
	}
}

// closeLateAttach 回收超时后才返回的 attach 连接（等 attach 的 goroutine
// 无法从 http.ReadResponse 中收回，但至少不让已建立的连接悬挂）。
func closeLateAttach(ch <-chan attachResult) {
	r := <-ch
	if r.resp.Conn != nil {
		r.resp.Close()
	}
}

// execStreamReattachBackoff 重挂前退避：0.5s、1s、2s（超出上限时按 2s）
func execStreamReattachBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 3 {
		attempt = 3
	}
	return 500 * time.Millisecond << (attempt - 1)
}

// streamOutcome 一轮 attach 读取结束后的处置
// （抽成纯函数便于单测：截断判定不依赖真 docker）。
type streamOutcome int

const (
	// streamOutcomeDone 命令已退出 ⇒ 正常结束
	streamOutcomeDone streamOutcome = iota
	// streamOutcomeReattach 进程仍在运行而流已断 ⇒ 输出流被截断，重挂续读
	streamOutcomeReattach
	// streamOutcomeGiveUp 已用尽重挂次数仍中断 ⇒ 判失败，绝不静默当成功
	streamOutcomeGiveUp
)

// classifyStreamEnd 判定"流结束"的处置：仅当进程仍在运行时才视为截断
// （正常结束时进程必然已退出，EOF 与断流的唯一可区分依据）。
func classifyStreamEnd(running bool, reattached, max int) streamOutcome {
	if !running {
		return streamOutcomeDone
	}
	if reattached >= max {
		return streamOutcomeGiveUp
	}
	return streamOutcomeReattach
}

// execStreamSettleWindow 进程刚退出时 docker 可能瞬时仍报 Running=true；判定截断前
// 先观察一个窗口，避免把"正常结束"误判为断流（会造成多余重挂与 StreamTruncated 假阳性）。
const execStreamSettleWindow = 1500 * time.Millisecond

// execRunningSettled 查询 exec 是否仍在运行，但要求 Running=true 在 settle 窗口内持续成立：
// 一旦某次查询返回 !Running 立即返回 false（即命令已退出 = 正常结束）。
func (d *DockerExecutor) execRunningSettled(ctx context.Context, execID string, settle time.Duration) (bool, error) {
	deadline := time.Now().Add(settle)
	for {
		running, err := d.execRunning(ctx, execID)
		if err != nil {
			return false, err
		}
		if !running {
			return false, nil
		}
		if !time.Now().Before(deadline) {
			return true, nil
		}
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
}

// execRunning 查询 exec 是否仍在运行
func (d *DockerExecutor) execRunning(ctx context.Context, execID string) (bool, error) {
	inspectCtx, cancel := d.controlContext(ctx)
	defer cancel()
	resp, err := d.client.ContainerExecInspect(inspectCtx, execID)
	if err != nil {
		return false, err
	}
	return resp.Running, nil
}

// execInspect 查询 exec 终态（退出码）
func (d *DockerExecutor) execInspect(ctx context.Context, execID string) (container.ExecInspect, error) {
	inspectCtx, cancel := d.controlContext(ctx)
	defer cancel()
	return d.client.ContainerExecInspect(inspectCtx, execID)
}

// detectShell 检测容器中的shell
func (d *DockerExecutor) detectShell() string {
	// 根据镜像类型选择shell
	image := strings.ToLower(d.image)
	if strings.Contains(image, "alpine") || strings.Contains(image, "busybox") {
		return "/bin/sh"
	}
	return "/bin/bash"
}

// pullImageIfNeeded 检查并拉取镜像
func (d *DockerExecutor) pullImageIfNeeded(ctx context.Context, imageName string) error {
	// 镜像探测是控制面调用：daemon 无响应时必须在有限时间内失败，
	// 不能卡在这里等一个永远不会到的响应
	inspectCtx, cancel := d.controlContext(ctx)
	defer cancel()

	if _, err := d.client.ImageInspect(inspectCtx, imageName); err == nil {
		return nil
	} else if !isImageNotFound(err) {
		// daemon 不可达/无响应/鉴权失败等：直接失败，不再误入"拉取"分支
		//（误入会让同样的超时再叠加一次，并把错误误导成"拉取失败"）
		return fmt.Errorf("failed to inspect image %s: %w", imageName, d.daemonErrHint(err))
	}

	// 镜像确实不存在才拉取。拉取进度流可能持续很久，不能用控制面超时约束
	// 响应体读取；但"daemon 是否响应"必须有界——用独立计时等 ImagePull
	// 拿到响应头（拿到后计时释放，body 读取不受限）。
	reader, err := d.pullImage(ctx, imageName)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	// 等待拉取完成（读取所有输出）
	_, _ = io.Copy(io.Discard, reader)

	return nil
}

// pullResult ImagePull 结果（供超时后延迟回收使用）
type pullResult struct {
	reader io.ReadCloser
	err    error
}

// pullImage 发起镜像拉取，并对"daemon 是否响应拉取请求"加超时。
// ImagePull 返回的是进度流（需长时间读取），不能直接用带 deadline 的 ctx
// 调用（超时会连带掐断 body）。这里在独立 goroutine 中等响应，
// 超时即判失败；超时后才返回的流由 closeLatePull 回收。
func (d *DockerExecutor) pullImage(ctx context.Context, imageName string) (io.ReadCloser, error) {
	timeout := d.daemonResponseTimeout()
	ch := make(chan pullResult, 1)
	go func() {
		reader, err := d.client.ImagePull(ctx, imageName, image.PullOptions{})
		ch <- pullResult{reader: reader, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf("failed to pull image %s: %w", imageName, d.daemonErrHint(r.err))
		}
		return r.reader, nil
	case <-ctx.Done():
		go closeLatePull(ch)
		return nil, ctx.Err()
	case <-timer.C:
		go closeLatePull(ch)
		return nil, fmt.Errorf(
			"docker daemon not responding (host=%q): image pull %s timed out after %s",
			d.host, imageName, timeout)
	}
}

// closeLatePull 回收超时后才建立的拉取流
func closeLatePull(ch <-chan pullResult) {
	r := <-ch
	if r.reader != nil {
		_ = r.reader.Close()
	}
}

// isImageNotFound 判断 ImageInspect 的错误是否表示"镜像不存在"。
// 只有镜像不存在才应转入拉取；daemon 不可达/超时/鉴权失败等必须直接失败，
// 否则会用一个同样会超时的拉取请求掩盖真正的连接问题。
func isImageNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errdefs.IsNotFound(err) {
		return true
	}
	// 兜底：个别 daemon/镜像代理对缺失镜像返回非标准文案；
	// 只匹配明确的"镜像不存在"语义，不用宽泛的 "not found"（避免把隧道/代理的 404 页面
	// 误判为镜像缺失而转去拉取，掩盖真正的连接问题）
	return strings.Contains(strings.ToLower(err.Error()), "no such image")
}

// waitForContainer 等待容器启动完成
func (d *DockerExecutor) waitForContainer(ctx context.Context) error {
	for i := 0; i < 30; i++ {
		containerJSON, err := d.client.ContainerInspect(ctx, d.containerID)
		if err != nil {
			return err
		}

		if containerJSON.State.Running {
			return nil
		}

		// 容器已退出（启动失败），直接报错，不再重试
		if containerJSON.State.Status == "exited" || containerJSON.State.Status == "dead" {
			if containerJSON.State.Error != "" {
				return fmt.Errorf("container exited with code %d: %s", containerJSON.State.ExitCode, containerJSON.State.Error)
			}
			return fmt.Errorf("container exited with code %d", containerJSON.State.ExitCode)
		}

		// 容器仍在启动中（如 created/restarting），等待后重试
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	return fmt.Errorf("timeout waiting for container to start")
}

// resolveImageName 解析完整的镜像名称
func (d *DockerExecutor) resolveImageName() string {
	if d.registry == "" || strings.Contains(d.image, "/") {
		return d.image
	}
	return fmt.Sprintf("%s/%s", d.registry, d.image)
}

// buildEnvList 构建环境变量列表
func (d *DockerExecutor) buildEnvList() []string {
	envList := make([]string, 0, len(d.env))
	for k, v := range d.env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	return envList
}

// buildMounts 构建挂载配置
func (d *DockerExecutor) buildMounts() []mount.Mount {
	mounts := make([]mount.Mount, 0, len(d.volumes))
	for hostPath, containerPath := range d.volumes {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: hostPath,
			Target: containerPath,
		})
	}
	return mounts
}

// setImage 设置镜像
func (d *DockerExecutor) setImage(image string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.image = image
}

// setWorkdir 设置工作目录
func (d *DockerExecutor) setWorkdir(workdir string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.workdir = workdir
}

// setEnv 设置环境变量
func (d *DockerExecutor) setEnv(key, value string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.env[key] = value
}

// setVolume 设置卷挂载
func (d *DockerExecutor) setVolume(hostPath, containerPath string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.volumes[hostPath] = containerPath
}

// setNetwork 设置网络
func (d *DockerExecutor) setNetwork(network string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.network = network
}

// setRegistry 设置镜像仓库
func (d *DockerExecutor) setRegistry(registry string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.registry = registry
}

// setHost 设置 Docker daemon 地址（如 tcp://192.168.1.10:2375、ssh://user@host）。
// 空字符串表示从环境变量读取（DOCKER_HOST 等）。
func (d *DockerExecutor) setHost(host string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.host = host
}

// setTLSVerify 设置是否对 daemon 连接启用 TLS 校验
func (d *DockerExecutor) setTLSVerify(verify bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tlsVerify = verify
}

// setCertPath 设置 TLS 证书目录（目录内需含 ca.pem / cert.pem / key.pem）
func (d *DockerExecutor) setCertPath(certPath string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.certPath = certPath
}

// setTTY 设置是否启用 TTY 模式
func (d *DockerExecutor) setTTY(enabled bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tty = enabled
}

// setTTYSize 设置 TTY 终端尺寸
func (d *DockerExecutor) setTTYSize(width, height uint) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ttyWidth = width
	d.ttyHeight = height
}

// setDaemonTimeout 设置 daemon 控制面请求响应超时（<=0 表示用默认值）
func (d *DockerExecutor) setDaemonTimeout(timeout time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.daemonTimeout = timeout
}

// GetContainerID 获取容器ID
func (d *DockerExecutor) GetContainerID() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.containerID
}

// GetRuntimeInfo 获取运行时信息
func (d *DockerExecutor) GetRuntimeInfo() map[string]any {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return map[string]any{
		"containerId": d.containerID,
		"image":       d.image,
		"network":     d.network,
		"workdir":     d.workdir,
		"registry":    d.registry,
	}
}

// GetInstanceId 获取实例ID（容器ID）
func (d *DockerExecutor) GetInstanceId() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.containerID
}

// GetType 获取executor类型
func (d *DockerExecutor) GetType() string {
	return "docker"
}

// ConnectionInfo Docker daemon 连接测试结果
// 用于 studio 等调用方的「连接状态测试」功能，不创建容器，仅探测 daemon 可达性。
type ConnectionInfo struct {
	ServerVersion string `json:"serverVersion"` // daemon 的 Docker 版本
	APIVersion    string `json:"apiVersion"`    // 协商后的 API 版本
	OS            string `json:"os"`            // daemon 所在操作系统
	Arch          string `json:"arch"`          // daemon 架构
	Name          string `json:"name"`          // daemon 节点名
	LatencyMs     int64  `json:"latencyMs"`     // Ping 往返耗时（毫秒）
}

// TestConnection 测试与 Docker daemon 的连接状态。
// 只做 Ping + ServerVersion，不拉取镜像也不创建容器；失败时返回带原因的 error。
func (d *DockerExecutor) TestConnection(ctx context.Context) (*ConnectionInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.ensureClient(); err != nil {
		return nil, err
	}

	// 调用方未给 deadline 时也必须有界，避免直接 API 调用挂死
	testCtx, cancel := d.controlContext(ctx)
	defer cancel()

	start := time.Now()
	if _, err := d.client.Ping(testCtx); err != nil {
		return nil, fmt.Errorf("docker daemon ping failed (host=%q): %w", d.host, err)
	}
	latency := time.Since(start).Milliseconds()

	info := &ConnectionInfo{LatencyMs: latency}
	if ver, err := d.client.ServerVersion(testCtx); err == nil {
		info.ServerVersion = ver.Version
		info.APIVersion = ver.APIVersion
		info.OS = ver.Os
		info.Arch = ver.Arch
		info.Name = ver.Platform.Name
	}
	return info, nil
}

// TestConnectionWithConfig 按配置项构造一个临时 Docker 执行器并测试 daemon 连接。
// config 支持的键与 DockerAdapter 一致（host/tlsVerify/certPath 等，与连接无关的
// image/network/volumes 等会被忽略）。不创建容器，调用方无需 Prepare/Destruction。
func TestConnectionWithConfig(ctx context.Context, config map[string]any) (*ConnectionInfo, error) {
	exec, err := NewDockerExecutor()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker executor: %w", err)
	}
	if err := applyConfigToExecutor(config, exec); err != nil {
		return nil, err
	}
	return exec.TestConnection(ctx)
}

// safeSend 安全地发送数据到 channel，如果 channel 已关闭则忽略
func safeSend(ch chan<- any, value any) {
	defer func() {
		if r := recover(); r != nil {
			_ = r // channel 已关闭，忽略
		}
	}()
	ch <- value
}

// 确保DockerExecutor实现了Executor接口和ExecutorInfoProvider接口
var _ executor.Executor = (*DockerExecutor)(nil)
var _ executor.ExecutorInfoProvider = (*DockerExecutor)(nil)

// ===== 容器内输出兜底（tee）：仅用于声明了 extract 的步骤 =====
//
// 动机：docker attach 流被中途掐断时客户端读到的是干净 EOF，与"命令正常结束"无法
// 区分（见 executeCommandInContainerStreaming），重挂也可能接不回中间缺的那段。
// 对"输出块要被下游消费"的节点，光靠流不可靠 —— 让输出同时落盘在容器内，
// 断了就按字节前缀补齐尾部（exec 364 事故的直接兜底）。

// captureSeq 保证同一容器内多个步骤/节点的临时文件互不覆盖
var captureSeq atomic.Uint64

// newCaptureTag 生成容器内临时文件名后缀：步骤名（清洗）+ pid + 自增序号
func newCaptureTag(stepName string) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, stepName)
	if name == "" {
		name = "step"
	}
	if len(name) > 24 {
		name = name[:24]
	}
	return fmt.Sprintf("%s-%d-%d", name, os.Getpid(), captureSeq.Add(1))
}

func captureLogPath(tag string) string { return "/tmp/.flowx-capture-" + tag + ".log" }
func captureRCPath(tag string) string  { return "/tmp/.flowx-capture-" + tag + ".rc" }

// wrapCommandForCapture 把命令包成"输出落盘 + 退出码走文件"的形式：
//
//	{ { <cmd>; } 2>&1; echo $? > <rc>; } | tee <log>; __flowx_rc=$(cat <rc> 2>/dev/null); exit ${__flowx_rc:-1}
//
// 要点：
//   - stderr 在**内层组**上整体重定向进管道，多行命令的每一行 stderr 都进日志
//     （只重定向最后一行会让日志比流少字节，破坏"日志前缀=已送达"的补发前提）
//   - `echo $?` 紧跟内层组：拿到的仍是命令退出码（不是 tee 的）
//   - 末尾显式 exit 让 exec 退出码回到命令语义，`execInspect` 仍然可信
//   - rc 文件缺失/不可读时按 1 处理：宁可失败也不要静默成功
func wrapCommandForCapture(command, tag string) string {
	logPath, rcPath := captureLogPath(tag), captureRCPath(tag)
	return fmt.Sprintf(
		"{ { %s\n} 2>&1; echo $? > %s; } | tee %s; "+
			"__flowx_rc=$(cat %s 2>/dev/null); exit ${__flowx_rc:-1}",
		command, rcPath, logPath, rcPath)
}

// replayCapturedTail 断流兜底：把容器内日志中"实时流没送到"的尾部补发出去。
//
// streamBytes 是已从 attach 流读到的字节数，即日志的已送达前缀长度；日志与流同为
// tee 的同一次读取，故流内容恒为日志的前缀（tee 先写文件后写 stdout）。用
// `tail -c +N` 从该前缀之后开始输出；若已送达字节数已超过日志长度（进程仍在运行、
// tee 尚未 flush），tail 输出为空 —— 只少补、不重复。顺带清理临时文件。
func (d *DockerExecutor) replayCapturedTail(ctx context.Context, tag string, streamBytes int, outputCallback func([]byte)) {
	if tag == "" {
		return
	}
	logPath, rcPath := captureLogPath(tag), captureRCPath(tag)
	if streamBytes < 0 {
		streamBytes = 0
	}
	cmd := fmt.Sprintf("tail -c +%d %s 2>/dev/null; rm -f %s %s 2>/dev/null",
		streamBytes+1, logPath, logPath, rcPath)
	out, err := d.runContainerCommand(ctx, cmd)
	if err != nil {
		if outputCallback != nil {
			outputCallback([]byte(fmt.Sprintf("[flowx] 从容器内补齐输出失败: %v\n", err)))
		}
		return
	}
	if out != "" {
		if outputCallback != nil {
			outputCallback([]byte(
				"[flowx] docker exec 流曾中断，以下为从容器内日志补齐的缺失输出：\n"))
			outputCallback([]byte(out))
		}
		fmt.Printf("docker exec captured tail replayed: tag=%s from=%d bytes=%d\n",
			tag, streamBytes, len(out))
	}
}

// cleanupCapture 清理容器内临时文件（未发生断流时调用）
func (d *DockerExecutor) cleanupCapture(ctx context.Context, tag string) {
	if tag == "" {
		return
	}
	if _, err := d.runContainerCommand(ctx,
		fmt.Sprintf("rm -f %s %s 2>/dev/null", captureLogPath(tag), captureRCPath(tag))); err != nil {
		fmt.Printf("Warning: 清理容器内输出兜底文件失败: %v\n", err)
	}
}

// runContainerCommand 在容器内执行一条短命令并返回其合并输出（非流式，兜底路径专用）。
// 不复用流式实现：它要注册取消回调、维护重挂连接与输入通道，代价远高于收益。
func (d *DockerExecutor) runContainerCommand(ctx context.Context, command string) (string, error) {
	d.mu.RLock()
	containerID := d.containerID
	d.mu.RUnlock()
	if containerID == "" {
		return "", fmt.Errorf("container not prepared")
	}

	// 创建 exec 是控制面调用：daemon 无响应时有界失败
	createCtx, createCancel := d.controlContext(ctx)
	execResp, err := d.client.ContainerExecCreate(createCtx, containerID, container.ExecOptions{
		Cmd:          []string{d.detectShell(), "-c", command},
		AttachStdout: true,
		AttachStderr: true,
	})
	createCancel()
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}
	attachResp, err := d.attachExec(ctx, execResp.ID)
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	// 短命令（兜底路径）加读超时：daemon 中途不响应时不至于卡死
	if attachResp.Conn != nil {
		_ = attachResp.Conn.SetReadDeadline(time.Now().Add(d.daemonResponseTimeout()))
	}

	var out strings.Builder
	if d.tty {
		// TTY 模式是裸流
		if _, err := io.Copy(&out, attachResp.Reader); err != nil && err != io.EOF {
			return out.String(), err
		}
		return out.String(), nil
	}
	if _, err := stdcopy.StdCopy(&out, &out, attachResp.Reader); err != nil && err != io.EOF {
		return out.String(), err
	}
	return out.String(), nil
}
