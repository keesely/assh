package virtual

import (
	"context"
	"fmt"
	"io"
	"strings"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// InputHandler 处理 SSH 会话中的输入，拦截虚拟命令。
type InputHandler struct {
	service port.VirtualCommandService
}

// NewInputHandler 创建 InputHandler 实例。
func NewInputHandler(service port.VirtualCommandService) *InputHandler {
	return &InputHandler{
		service: service,
	}
}

// HandleInput 处理用户输入，如果是虚拟命令则执行，否则返回 false。
// 返回值：
// - handled: 是否是虚拟命令（已处理）
// - err: 执行错误
func (h *InputHandler) HandleInput(ctx context.Context, input string, session port.SSHSession) (handled bool, err error) {
	// 检查是否是虚拟命令
	if !h.service.IsVirtualCommand(input) {
		return false, nil
	}

	// 执行虚拟命令
	result, err := h.service.Execute(ctx, input, session)
	if err != nil {
		// 显示错误信息
		fmt.Printf("[ERROR] %v\n", err)
		return true, nil
	}

	// 显示结果
	if result != nil {
		h.displayResult(result)
	}

	return true, nil
}

// HandleInputWithWriter 处理用户输入，输出到指定 writer。
func (h *InputHandler) HandleInputWithWriter(ctx context.Context, input string, session port.SSHSession, writer io.Writer) (handled bool, err error) {
	// 检查是否是虚拟命令
	if !h.service.IsVirtualCommand(input) {
		return false, nil
	}

	// 执行虚拟命令
	result, err := h.service.Execute(ctx, input, session)
	if err != nil {
		// 显示错误信息
		fmt.Fprintf(writer, "[ERROR] %v\n", err)
		return true, nil
	}

	// 显示结果
	if result != nil {
		h.displayResultToWriter(result, writer)
	}

	return true, nil
}

// displayResult 显示命令执行结果。
func (h *InputHandler) displayResult(result *domain.VirtualCommandResult) {
	if result == nil {
		return
	}

	// 显示输出
	if result.Output != "" {
		fmt.Println(result.Output)
	}

	// 显示错误
	if result.Error != "" {
		fmt.Printf("[ERROR] %s\n", result.Error)
	}
}

// displayResultToWriter 显示命令执行结果到指定 writer。
func (h *InputHandler) displayResultToWriter(result *domain.VirtualCommandResult, writer io.Writer) {
	if result == nil {
		return
	}

	// 显示输出
	if result.Output != "" {
		fmt.Fprintln(writer, result.Output)
	}

	// 显示错误
	if result.Error != "" {
		fmt.Fprintf(writer, "[ERROR] %s\n", result.Error)
	}
}

// IsVirtualCommand 检查输入是否是虚拟命令。
func (h *InputHandler) IsVirtualCommand(input string) bool {
	return h.service.IsVirtualCommand(input)
}

// GetCommandHint 获取命令提示（用于自动补全）。
func (h *InputHandler) GetCommandHint(input string) []string {
	if !strings.HasPrefix(input, "@") {
		return nil
	}

	// 获取所有命令
	commands := h.service.ListCommands()
	var hints []string

	// 过滤匹配的命令
	prefix := input[1:] // 移除 @
	for _, cmd := range commands {
		if strings.HasPrefix(cmd.Name(), prefix) {
			hints = append(hints, "@"+cmd.Name())
		}
	}

	return hints
}
