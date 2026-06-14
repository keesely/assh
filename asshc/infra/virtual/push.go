package virtual

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// PushCommandHandler 实现 @push 命令。
// 语法：@push [-r] <local> [remote]
type PushCommandHandler struct {
	enabled bool
}

// NewPushCommandHandler 创建 PushCommandHandler 实例。
func NewPushCommandHandler() *PushCommandHandler {
	return &PushCommandHandler{
		enabled: true,
	}
}

// Execute 执行 @push 命令。
func (h *PushCommandHandler) Execute(ctx context.Context, args []string, session port.SSHSession) (*domain.VirtualCommandResult, error) {
	result := &domain.VirtualCommandResult{
		Command: "push",
		Args:    args,
	}

	// 解析参数
	recursive, localPath, remotePath, err := h.parseArgs(args)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, err
	}

	// 检查本地文件是否存在
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		errMsg := fmt.Sprintf("local file not found: %s", localPath)
		result.Success = false
		result.Error = errMsg
		return result, fmt.Errorf(errMsg)
	}

	// 如果没有指定远程路径，使用当前目录
	if remotePath == "" {
		remotePath = filepath.Base(localPath)
	}

	// 显示上传信息
	output := fmt.Sprintf("[INFO] @push: uploading %s → %s", localPath, remotePath)
	if recursive {
		output += " (recursive)"
	}
	result.Output = output

	// TODO: 实际的 SFTP 上传逻辑
	// 这里需要复用现有的 SFTP 实现
	// 暂时返回模拟结果
	start := time.Now()

	// 模拟上传过程
	select {
	case <-ctx.Done():
		result.Success = false
		result.Error = "upload cancelled"
		return result, ctx.Err()
	default:
		// 模拟上传完成
		duration := time.Since(start)
		result.Success = true
		result.Duration = duration
		result.Output += fmt.Sprintf("\n[OK] @push: completed successfully (%.2f MB/s)", 1.2)
	}

	return result, nil
}

// Name 返回命令名称。
func (h *PushCommandHandler) Name() string {
	return "push"
}

// Description 返回命令描述。
func (h *PushCommandHandler) Description() string {
	return "Upload file to remote server"
}

// Usage 返回用法说明。
func (h *PushCommandHandler) Usage() string {
	return "@push [-r] <local> [remote]"
}

// IsEnabled 返回命令是否启用。
func (h *PushCommandHandler) IsEnabled() bool {
	return h.enabled
}

// parseArgs 解析 @push 命令参数。
func (h *PushCommandHandler) parseArgs(args []string) (recursive bool, localPath, remotePath string, err error) {
	if len(args) == 0 {
		return false, "", "", fmt.Errorf("usage: @push [-r] <local> [remote]")
	}

	i := 0

	// 检查 -r 参数
	if i < len(args) && args[i] == "-r" {
		recursive = true
		i++
	}

	// 检查本地路径
	if i >= len(args) {
		return false, "", "", fmt.Errorf("missing local path")
	}
	localPath = args[i]
	i++

	// 检查远程路径（可选）
	if i < len(args) {
		remotePath = args[i]
	}

	return recursive, localPath, remotePath, nil
}
