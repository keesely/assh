// Package domain 定义 ASSH 应用的核心领域模型。
//
// 本包包含最基础的业务实体（Server、Auth）和通用错误定义，
// 不依赖任何外部基础设施。其他层（port、service、infra）均
// 引用本包定义的类型。
package domain

import (
	"fmt"
	"time"
)

// VirtualCommand 表示一个虚拟命令的定义。
// 虚拟命令是 SSH 客户端的本地命令扩展，以 @ 为前缀触发。
type VirtualCommand struct {
	Name        string        `json:"name"`        // 命令名称（不含前缀）
	Description string        `json:"description"` // 命令描述
	Usage       string        `json:"usage"`       // 用法说明
	Handler     string        `json:"handler"`     // 处理器名称
	Timeout     time.Duration `json:"timeout"`     // 执行超时
	Enabled     bool          `json:"enabled"`     // 是否启用
	Builtin     bool          `json:"builtin"`     // 是否内置命令
}

// VirtualCommandConfig 表示虚拟命令的配置。
type VirtualCommandConfig struct {
	Builtin  map[string]BuiltinCommandConfig  `yaml:"builtin"`  // 内置命令配置
	Custom   map[string]CustomCommandConfig   `yaml:"custom"`   // 自定义命令配置
	Settings VirtualCommandSettings           `yaml:"settings"` // 全局设置
}

// BuiltinCommandConfig 表示内置命令的配置。
type BuiltinCommandConfig struct {
	Enabled bool          `yaml:"enabled"` // 是否启用
	Timeout time.Duration `yaml:"timeout"` // 超时时间
}

// CustomCommandConfig 表示自定义命令的配置。
type CustomCommandConfig struct {
	Description string        `yaml:"description"` // 命令描述
	Script      string        `yaml:"script"`      // 脚本内容
	Timeout     time.Duration `yaml:"timeout"`     // 超时时间
}

// VirtualCommandSettings 表示虚拟命令的全局设置。
type VirtualCommandSettings struct {
	OnError string `yaml:"on-error"` // 错误处理：continue | exit
	Output  string `yaml:"output"`   // 输出格式：color | plain | json
	History bool   `yaml:"history"`  // 是否记录历史
}

// VirtualCommandResult 表示虚拟命令的执行结果。
type VirtualCommandResult struct {
	Command   string        `json:"command"`   // 执行的命令
	Args      []string      `json:"args"`      // 命令参数
	Success   bool          `json:"success"`   // 是否成功
	Output    string        `json:"output"`    // 输出内容
	Error     string        `json:"error"`     // 错误信息
	Duration  time.Duration `json:"duration"`  // 执行时长
	ExitCode  int           `json:"exitCode"`  // 退出码
}

// VirtualCommandError 表示虚拟命令相关的错误。
type VirtualCommandError struct {
	Command string
	Message string
	Err     error
}

func (e *VirtualCommandError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("virtual command %s: %s: %v", e.Command, e.Message, e.Err)
	}
	return fmt.Sprintf("virtual command %s: %s", e.Command, e.Message)
}

// 预定义的虚拟命令错误。
var (
	ErrCommandNotFound        = &VirtualCommandError{Message: "command not found"}
	ErrCommandDisabled        = &VirtualCommandError{Message: "command is disabled"}
	ErrInvalidArgs            = &VirtualCommandError{Message: "invalid arguments"}
	ErrVirtualTimeout         = &VirtualCommandError{Message: "command timeout"}
	ErrVirtualExecutionFailed = &VirtualCommandError{Message: "execution failed"}
)

// IsVirtualCommand 检查输入是否是虚拟命令（以 @ 开头）。
func IsVirtualCommand(input string) bool {
	return len(input) > 0 && input[0] == '@'
}

// ParseVirtualCommand 解析虚拟命令输入，返回命令名和参数。
// 输入格式：@command [args...]
func ParseVirtualCommand(input string) (command string, args []string) {
	if !IsVirtualCommand(input) {
		return "", nil
	}

	// 移除 @ 前缀
	input = input[1:]

	// 简单的空格分割（后续可以支持引号）
	parts := splitArgs(input)
	if len(parts) == 0 {
		return "", nil
	}

	return parts[0], parts[1:]
}

// splitArgs 简单的参数分割（支持双引号）。
func splitArgs(input string) []string {
	var args []string
	var current string
	inQuote := false

	for _, ch := range input {
		switch {
		case ch == '"':
			inQuote = !inQuote
		case ch == ' ' && !inQuote:
			if current != "" {
				args = append(args, current)
				current = ""
			}
		default:
			current += string(ch)
		}
	}

	if current != "" {
		args = append(args, current)
	}

	return args
}
