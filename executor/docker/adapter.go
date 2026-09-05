package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/LerkoX/flowx/executor"
)

// DockerAdapter Docker适配器实现
type DockerAdapter struct {
	config map[string]any
}

// NewDockerAdapter 创建新的Docker适配器
func NewDockerAdapter() *DockerAdapter {
	return &DockerAdapter{
		config: make(map[string]any),
	}
}

// Config 配置适配器
// 支持的配置项：
//   - host: Docker daemon 地址 string（如 "tcp://192.168.1.10:2375"、"ssh://user@host"；
//     未设置时读取环境变量 DOCKER_HOST，默认本机 unix socket）
//   - tlsVerify: 是否对 daemon 连接启用 TLS 校验 bool（配合 host 使用）
//   - certPath: TLS 证书目录 string（含 ca.pem/cert.pem/key.pem，默认 "~/.docker"）
//   - image: 容器镜像 string（未设置时默认 "alpine:latest"；studio 展开器会把节点的
//     image 字段注入到该执行器条目的 config.image，使节点声明镜像真正生效）
//   - registry: 镜像仓库地址
//   - network: Docker网络模式
//   - volumes: 卷挂载列表 []string{"/host:/container"}（注意：远程 daemon 时为远端机器路径）
//   - workdir: 工作目录
//   - env: 环境变量 map[string]string
//   - tty: 是否启用 TTY 模式 bool
//   - ttyWidth: TTY 终端宽度 int（默认 80）
//   - ttyHeight: TTY 终端高度 int（默认 24）
func (a *DockerAdapter) Config(ctx context.Context, config map[string]any) error {
	a.config = config
	return nil
}

// 确保DockerAdapter实现了Adapter接口
var _ executor.Adapter = (*DockerAdapter)(nil)

// parseVolume 解析卷挂载字符串
// 支持的格式：
//   - /host/path:/container/path
//   - /host/path:/container/path:ro
func parseVolume(volume string) (hostPath, containerPath string, err error) {
	parts := strings.SplitN(volume, ":", 3)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid volume format, expected 'host:container'")
	}

	hostPath = parts[0]
	containerPath = parts[1]

	// 验证路径不为空
	if hostPath == "" || containerPath == "" {
		return "", "", fmt.Errorf("host path and container path cannot be empty")
	}

	return hostPath, containerPath, nil
}
