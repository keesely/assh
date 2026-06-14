package virtual

import (
	"context"
	"fmt"
	"strings"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// HelpCommandHandler 实现 @help 命令。
// 语法：@help [command]
type HelpCommandHandler struct {
	enabled bool
	service port.VirtualCommandService
}

// NewHelpCommandHandler 创建 HelpCommandHandler 实例。
func NewHelpCommandHandler(service port.VirtualCommandService) *HelpCommandHandler {
	return &HelpCommandHandler{
		enabled: true,
		service: service,
	}
}

// Execute 执行 @help 命令。
func (h *HelpCommandHandler) Execute(ctx context.Context, args []string, session port.SSHSession) (*domain.VirtualCommandResult, error) {
	result := &domain.VirtualCommandResult{
		Command: "help",
		Args:    args,
	}

	var output string

	if len(args) == 0 {
		// 显示所有命令列表
		output = h.showAllCommands()
	} else {
		// 显示指定命令的帮助
		output = h.showCommandHelp(args[0])
	}

	result.Success = true
	result.Output = output
	return result, nil
}

// Name 返回命令名称。
func (h *HelpCommandHandler) Name() string {
	return "help"
}

// Description 返回命令描述。
func (h *HelpCommandHandler) Description() string {
	return "Show help information"
}

// Usage 返回用法说明。
func (h *HelpCommandHandler) Usage() string {
	return "@help [command]"
}

// IsEnabled 返回命令是否启用。
func (h *HelpCommandHandler) IsEnabled() bool {
	return h.enabled
}

// showAllCommands 显示所有命令列表。
func (h *HelpCommandHandler) showAllCommands() string {
	var sb strings.Builder

	sb.WriteString("Available virtual commands:\n")
	sb.WriteString("─────────────────────────────────────────\n")

	// 获取所有命令
	commands := h.service.ListCommands()

	// 按名称排序
	for _, cmd := range commands {
		sb.WriteString(fmt.Sprintf("  @%-12s %s\n", cmd.Name(), cmd.Description()))
	}

	sb.WriteString("─────────────────────────────────────────\n")
	sb.WriteString("Use '@help <command>' for more information.\n")

	return sb.String()
}

// showCommandHelp 显示指定命令的帮助。
func (h *HelpCommandHandler) showCommandHelp(commandName string) string {
	// 获取命令
	cmd, err := h.service.GetCommand(commandName)
	if err != nil {
		return fmt.Sprintf("[ERROR] @help: command not found: %s", commandName)
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Command: @%s\n", cmd.Name()))
	sb.WriteString(fmt.Sprintf("Description: %s\n", cmd.Description()))
	sb.WriteString(fmt.Sprintf("Usage: %s\n", cmd.Usage()))

	// 显示是否启用
	if cmd.IsEnabled() {
		sb.WriteString("Status: enabled\n")
	} else {
		sb.WriteString("Status: disabled\n")
	}

	return sb.String()
}
