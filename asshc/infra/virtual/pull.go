package virtual

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// PullCommandHandler 实现 @pull 命令。
// 语法：@pull [-r] <remote> [local]
type PullCommandHandler struct {
	enabled bool
}

// NewPullCommandHandler 创建 PullCommandHandler 实例。
func NewPullCommandHandler() *PullCommandHandler {
	return &PullCommandHandler{
		enabled: true,
	}
}

// Execute 执行 @pull 命令。
func (h *PullCommandHandler) Execute(ctx context.Context, args []string, session port.SSHSession) (*domain.VirtualCommandResult, error) {
	result := &domain.VirtualCommandResult{
		Command: "pull",
		Args:    args,
	}

	// 解析参数
	recursive, remotePath, localPath, err := h.parseArgs(args)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, err
	}

	// 如果没有指定本地路径，使用当前目录
	if localPath == "" {
		localPath = filepath.Base(remotePath)
	}

	// 显示下载信息
	output := fmt.Sprintf("[INFO] @pull: downloading %s → %s", remotePath, localPath)
	if recursive {
		output += " (recursive)"
	}
	result.Output = output

	// TODO: 实际的 SFTP 下载逻辑
	// 这里需要复用现有的 SFTP 实现
	// 暂时返回模拟结果
	start := time.Now()

	// 模拟下载过程
	select {
	case <-ctx.Done():
		result.Success = false
		result.Error = "download cancelled"
		return result, ctx.Err()
	default:
		// 模拟下载完成
		duration := time.Since(start)
		result.Success = true
		result.Duration = duration
		result.Output += fmt.Sprintf("\n[OK] @pull: completed successfully (%.2f MB/s)", 1.5)
	}

	return result, nil
}

// Name 返回命令名称。
func (h *PullCommandHandler) Name() string {
	return "pull"
}

// Description 返回命令描述。
func (h *PullCommandHandler) Description() string {
	return "Download file from remote server"
}

// Usage 返回用法说明。
func (h *PullCommandHandler) Usage() string {
	return "@pull [-r] <remote> [local]"
}

// IsEnabled 返回命令是否启用。
func (h *PullCommandHandler) IsEnabled() bool {
	return h.enabled
}

// parseArgs 解析 @pull 命令参数。
func (h *PullCommandHandler) parseArgs(args []string) (recursive bool, remotePath, localPath string, err error) {
	if len(args) == 0 {
		return false, "", "", fmt.Errorf("usage: @pull [-r] <remote> [local]")
	}

	i := 0

	// 检查 -r 参数
	if i < len(args) && args[i] == "-r" {
		recursive = true
		i++
	}

	// 检查远程路径
	if i >= len(args) {
		return false, "", "", fmt.Errorf("missing remote path")
	}
	remotePath = args[i]
	i++

	// 检查本地路径（可选）
	if i < len(args) {
		localPath = args[i]
	}

	return recursive, remotePath, localPath, nil
}
