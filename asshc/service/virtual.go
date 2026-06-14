package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// VirtualCommandService 实现虚拟命令服务。
// 负责管理虚拟命令的注册、查找和执行。
type VirtualCommandServiceImpl struct {
	mu       sync.RWMutex
	commands map[string]port.VirtualCommandHandler
	config   *domain.VirtualCommandConfig
}

// NewVirtualCommandService 创建 VirtualCommandService 实例。
func NewVirtualCommandService() *VirtualCommandServiceImpl {
	return &VirtualCommandServiceImpl{
		commands: make(map[string]port.VirtualCommandHandler),
	}
}

// Register 注册一个虚拟命令处理器。
func (s *VirtualCommandServiceImpl) Register(handler port.VirtualCommandHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := handler.Name()
	if name == "" {
		return fmt.Errorf("command name cannot be empty")
	}

	if _, exists := s.commands[name]; exists {
		return fmt.Errorf("command %q already registered", name)
	}

	s.commands[name] = handler
	return nil
}

// RegisterBuiltin 注册所有内置命令。
// 注意：此方法需要在外部注入具体的命令处理器。
func (s *VirtualCommandServiceImpl) RegisterBuiltin() error {
	// 内置命令注册需要在外部完成
	// 这里只是一个占位方法
	return nil
}

// Execute 执行虚拟命令。
func (s *VirtualCommandServiceImpl) Execute(ctx context.Context, input string, session port.SSHSession) (*domain.VirtualCommandResult, error) {
	// 解析输入
	command, args, err := s.ParseInput(input)
	if err != nil {
		return nil, err
	}

	// 获取命令处理器
	handler, err := s.GetCommand(command)
	if err != nil {
		return nil, err
	}

	// 检查命令是否启用
	if !handler.IsEnabled() {
		return nil, domain.ErrCommandDisabled
	}

	// 创建带超时的上下文
	timeout := s.getCommandTimeout(command)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 记录开始时间
	start := time.Now()

	// 执行命令
	result, err := handler.Execute(ctx, args, session)

	// 计算执行时长
	duration := time.Since(start)

	// 构建结果
	if result == nil {
		result = &domain.VirtualCommandResult{
			Command:  command,
			Args:     args,
			Duration: duration,
		}
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.ExitCode = 1
	} else {
		result.Success = true
		result.ExitCode = 0
	}

	return result, nil
}

// GetCommand 获取指定名称的命令处理器。
func (s *VirtualCommandServiceImpl) GetCommand(name string) (port.VirtualCommandHandler, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	handler, exists := s.commands[name]
	if !exists {
		return nil, &domain.VirtualCommandError{
			Command: name,
			Message: "command not found",
		}
	}

	return handler, nil
}

// ListCommands 列出所有已注册的命令。
func (s *VirtualCommandServiceImpl) ListCommands() []port.VirtualCommandHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()

	commands := make([]port.VirtualCommandHandler, 0, len(s.commands))
	for _, handler := range s.commands {
		commands = append(commands, handler)
	}

	return commands
}

// IsVirtualCommand 检查输入是否是虚拟命令。
func (s *VirtualCommandServiceImpl) IsVirtualCommand(input string) bool {
	return domain.IsVirtualCommand(input)
}

// ParseInput 解析虚拟命令输入。
func (s *VirtualCommandServiceImpl) ParseInput(input string) (command string, args []string, err error) {
	if !s.IsVirtualCommand(input) {
		return "", nil, fmt.Errorf("not a virtual command: %s", input)
	}

	command, args = domain.ParseVirtualCommand(input)
	if command == "" {
		return "", nil, fmt.Errorf("empty command name")
	}

	return command, args, nil
}

// getCommandTimeout 获取命令的超时时间。
func (s *VirtualCommandServiceImpl) getCommandTimeout(command string) time.Duration {
	// 默认超时 30 秒
	defaultTimeout := 30 * time.Second

	if s.config == nil {
		return defaultTimeout
	}

	// 检查内置命令配置
	if builtinConfig, exists := s.config.Builtin[command]; exists {
		if builtinConfig.Timeout > 0 {
			return builtinConfig.Timeout
		}
	}

	// 检查自定义命令配置
	if customConfig, exists := s.config.Custom[command]; exists {
		if customConfig.Timeout > 0 {
			return customConfig.Timeout
		}
	}

	return defaultTimeout
}

// SetConfig 设置虚拟命令配置。
func (s *VirtualCommandServiceImpl) SetConfig(config *domain.VirtualCommandConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// GetConfig 获取虚拟命令配置。
func (s *VirtualCommandServiceImpl) GetConfig() *domain.VirtualCommandConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}
