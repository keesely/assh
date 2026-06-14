package virtual

import (
	"context"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// StatusCommandHandler 实现 @status 命令。
type StatusCommandHandler struct {
	enabled bool
}

// NewStatusCommandHandler 创建 StatusCommandHandler 实例。
func NewStatusCommandHandler() *StatusCommandHandler {
	return &StatusCommandHandler{
		enabled: true,
	}
}

// Execute 执行 @status 命令。
func (h *StatusCommandHandler) Execute(ctx context.Context, args []string, session port.SSHSession) (*domain.VirtualCommandResult, error) {
	result := &domain.VirtualCommandResult{
		Command: "status",
		Args:    args,
	}

	// 获取连接状态
	var output string
	if session != nil {
		output = "Connection status: connected"
		// TODO: 获取更多连接信息
	} else {
		output = "Connection status: no active session"
	}

	result.Success = true
	result.Output = output
	return result, nil
}

// Name 返回命令名称。
func (h *StatusCommandHandler) Name() string {
	return "status"
}

// Description 返回命令描述。
func (h *StatusCommandHandler) Description() string {
	return "Show connection status"
}

// Usage 返回用法说明。
func (h *StatusCommandHandler) Usage() string {
	return "@status"
}

// IsEnabled 返回命令是否启用。
func (h *StatusCommandHandler) IsEnabled() bool {
	return h.enabled
}
