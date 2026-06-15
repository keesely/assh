package virtual

import (
	"context"
	"fmt"
	"runtime"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// VersionCommandHandler 实现 @version 命令。
type VersionCommandHandler struct {
	enabled bool
}

// NewVersionCommandHandler 创建 VersionCommandHandler 实例。
func NewVersionCommandHandler() *VersionCommandHandler {
	return &VersionCommandHandler{
		enabled: true,
	}
}

// Execute 执行 @version 命令。
func (h *VersionCommandHandler) Execute(ctx context.Context, args []string, session port.SSHSession) (*domain.VirtualCommandResult, error) {
	result := &domain.VirtualCommandResult{
		Command: "version",
		Args:    args,
	}

	output := fmt.Sprintf("assh version v2.2.0-preview.1 (%s/%s)", runtime.GOOS, runtime.GOARCH)
	result.Success = true
	result.Output = output
	return result, nil
}

// Name 返回命令名称。
func (h *VersionCommandHandler) Name() string {
	return "version"
}

// Description 返回命令描述。
func (h *VersionCommandHandler) Description() string {
	return "Show assh version"
}

// Usage 返回用法说明。
func (h *VersionCommandHandler) Usage() string {
	return "@version"
}

// IsEnabled 返回命令是否启用。
func (h *VersionCommandHandler) IsEnabled() bool {
	return h.enabled
}
