package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LerkoX/flowx/executor"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/client"
	"gopkg.in/yaml.v2"
)

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
	host              string // daemon 地址（tcp://… / ssh://… / unix://…），空表示从环境变量读取（DOCKER_HOST 等）
	tlsVerify         bool   // 是否启用 TLS 校验
	certPath          string // TLS 证书目录（含 ca.pem/cert.pem/key.pem），默认为 ~/.docker
	tty               bool   // 是否启用 TTY 模式
	ttyHeight         uint   // TTY 终端高度
	ttyWidth          uint   // TTY 终端宽度
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
		return fmt.Errorf("failed to pull image: %w", err)
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

	// 创建容器
	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, fmt.Sprintf("flowx-%d", time.Now().UnixNano()))
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	d.containerID = resp.ID

	// 启动容器
	if err := d.client.ContainerStart(ctx, d.containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// 等待容器启动完成
	if err := d.waitForContainer(ctx); err != nil {
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
			// 执行命令（携带步骤名称）
			d.executeCommandStreaming(execCtx, cmdWrapper.Command, cmdWrapper.StepName, cmdWrapper.Env, resultChan, inputChan)
		}
	}
}

// executeCommandStreaming 执行命令并实时流式输出
func (d *DockerExecutor) executeCommandStreaming(ctx context.Context, command string, stepName string, env map[string]string, resultChan chan<- any, inputChan <-chan []byte) {
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

	truncated, err := d.executeCommandInContainerStreaming(ctx, command, env, func(data []byte) {
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
// 返回值 truncated 表示 attach 读取流曾被中断（已尽力重挂续读，次数=truncated）；
// 重挂仍无法续接时返回错误——**绝不静默当成功**：日志流被截断会让节点输出块丢失，
// 下游节点随后报"缺参"，把排查方向带偏（exec 364 事故：cpolar 上的 docker exec
// 流被截断 → KSampler 绿灯但无输出 → VAEDecode 报 missing required parameter）。
func (d *DockerExecutor) executeCommandInContainerStreaming(ctx context.Context, command string, env map[string]string, outputCallback func([]byte), inputChan <-chan []byte, onInputRequest func(*executor.InputRequest)) (int, error) {
	d.mu.RLock()
	containerID := d.containerID
	d.mu.RUnlock()

	if containerID == "" {
		return 0, fmt.Errorf("container not prepared")
	}

	shell := d.detectShell()

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

	execResp, err := d.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return 0, fmt.Errorf("failed to create exec: %w", err)
	}

	attachResp, err := d.client.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{
		Tty: d.tty,
	})
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
		_ = d.client.ContainerExecResize(ctx, execResp.ID, container.ResizeOptions{
			Width:  d.ttyWidth,
			Height: d.ttyHeight,
		})
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

			newResp, aerr := d.client.ContainerExecAttach(ctx, execResp.ID,
				container.ExecAttachOptions{Tty: d.tty})
			if aerr != nil {
				close(done)
				wg.Wait()
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
	resp, err := d.client.ContainerExecInspect(ctx, execID)
	if err != nil {
		return false, err
	}
	return resp.Running, nil
}

// execInspect 查询 exec 终态（退出码）
func (d *DockerExecutor) execInspect(ctx context.Context, execID string) (container.ExecInspect, error) {
	return d.client.ContainerExecInspect(ctx, execID)
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
	// 检查镜像是否存在
	_, err := d.client.ImageInspect(ctx, imageName)
	if err == nil {
		return nil
	}

	// 镜像不存在，需要拉取
	reader, err := d.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer func() { _ = reader.Close() }()

	// 等待拉取完成（读取所有输出）
	_, _ = io.Copy(io.Discard, reader)

	return nil
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

	start := time.Now()
	if _, err := d.client.Ping(ctx); err != nil {
		return nil, fmt.Errorf("docker daemon ping failed (host=%q): %w", d.host, err)
	}
	latency := time.Since(start).Milliseconds()

	info := &ConnectionInfo{LatencyMs: latency}
	if ver, err := d.client.ServerVersion(ctx); err == nil {
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
