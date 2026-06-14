package port

import (
	"assh/asshc/domain"
	"context"
)

// VirtualCommandHandler 定义虚拟命令处理器的接口。
// 每个虚拟命令（如 @push, @pull）都需要实现此接口。
type VirtualCommandHandler interface {
	// Execute 执行虚拟命令。
	// ctx: 上下文，用于超时控制
	// args: 命令参数（不含命令名）
	// session: 当前 SSH 会话（可选，用于需要远程操作的命令）
	Execute(ctx context.Context, args []string, session SSHSession) (*domain.VirtualCommandResult, error)

	// Name 返回命令名称。
	Name() string

	// Description 返回命令描述。
	Description() string

	// Usage 返回用法说明。
	Usage() string

	// IsEnabled 返回命令是否启用。
	IsEnabled() bool
}

// VirtualCommandService 定义虚拟命令服务的接口。
// 负责管理虚拟命令的注册、查找和执行。
type VirtualCommandService interface {
	// Register 注册一个虚拟命令处理器。
	Register(handler VirtualCommandHandler) error

	// RegisterBuiltin 注册所有内置命令。
	RegisterBuiltin() error

	// Execute 执行虚拟命令。
	// input: 用户输入（如 "@push file.txt"）
	// session: 当前 SSH 会话
	Execute(ctx context.Context, input string, session SSHSession) (*domain.VirtualCommandResult, error)

	// GetCommand 获取指定名称的命令处理器。
	GetCommand(name string) (VirtualCommandHandler, error)

	// ListCommands 列出所有已注册的命令。
	ListCommands() []VirtualCommandHandler

	// IsVirtualCommand 检查输入是否是虚拟命令。
	IsVirtualCommand(input string) bool

	// ParseInput 解析虚拟命令输入。
	ParseInput(input string) (command string, args []string, err error)
}

// VirtualCommandConfigLoader 定义虚拟命令配置加载器的接口。
type VirtualCommandConfigLoader interface {
	// Load 加载虚拟命令配置。
	Load() (*domain.VirtualCommandConfig, error)

	// Save 保存虚拟命令配置。
	Save(config *domain.VirtualCommandConfig) error

	// GetBuiltinConfig 获取指定内置命令的配置。
	GetBuiltinConfig(name string) (*domain.BuiltinCommandConfig, error)

	// GetCustomConfig 获取指定自定义命令的配置。
	GetCustomConfig(name string) (*domain.CustomCommandConfig, error)
}
