package virtual

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"assh/asshc/domain"
	"gopkg.in/yaml.v3"
)

// ConfigLoader 实现虚拟命令配置加载器。
// 配置文件位置：~/.assh/v2/virtual.yml
type ConfigLoader struct {
	mu       sync.RWMutex
	config   *domain.VirtualCommandConfig
	filePath string
}

// NewConfigLoader 创建 ConfigLoader 实例。
// configPath: 配置文件路径，如果为空则使用默认路径。
func NewConfigLoader(configPath string) *ConfigLoader {
	if configPath == "" {
		configPath = getDefaultConfigPath()
	}

	return &ConfigLoader{
		filePath: configPath,
	}
}

// getDefaultConfigPath 获取默认配置文件路径。
func getDefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".assh", "v2", "virtual.yml")
}

// Load 加载虚拟命令配置。
func (l *ConfigLoader) Load() (*domain.VirtualCommandConfig, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 如果配置已加载，直接返回
	if l.config != nil {
		return l.config, nil
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(l.filePath); os.IsNotExist(err) {
		// 配置文件不存在，返回默认配置
		l.config = l.getDefaultConfig()
		return l.config, nil
	}

	// 读取配置文件
	data, err := os.ReadFile(l.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析 YAML
	var config domain.VirtualCommandConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 合并默认值
	l.mergeDefaults(&config)

	l.config = &config
	return l.config, nil
}

// Save 保存虚拟命令配置。
func (l *ConfigLoader) Save(config *domain.VirtualCommandConfig) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 确保目录存在
	dir := filepath.Dir(l.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// 序列化为 YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(l.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	l.config = config
	return nil
}

// GetBuiltinConfig 获取指定内置命令的配置。
func (l *ConfigLoader) GetBuiltinConfig(name string) (*domain.BuiltinCommandConfig, error) {
	config, err := l.Load()
	if err != nil {
		return nil, err
	}

	if builtinConfig, exists := config.Builtin[name]; exists {
		return &builtinConfig, nil
	}

	// 返回默认配置
	return &domain.BuiltinCommandConfig{
		Enabled: true,
		Timeout: 30 * 1000000000, // 30 秒
	}, nil
}

// GetCustomConfig 获取指定自定义命令的配置。
func (l *ConfigLoader) GetCustomConfig(name string) (*domain.CustomCommandConfig, error) {
	config, err := l.Load()
	if err != nil {
		return nil, err
	}

	if customConfig, exists := config.Custom[name]; exists {
		return &customConfig, nil
	}

	return nil, fmt.Errorf("custom command %q not found", name)
}

// getDefaultConfig 获取默认配置。
func (l *ConfigLoader) getDefaultConfig() *domain.VirtualCommandConfig {
	return &domain.VirtualCommandConfig{
		Builtin: map[string]domain.BuiltinCommandConfig{
			"push": {
				Enabled: true,
				Timeout: 30 * 1000000000, // 30 秒
			},
			"pull": {
				Enabled: true,
				Timeout: 30 * 1000000000, // 30 秒
			},
			"run": {
				Enabled: true,
				Timeout: 60 * 1000000000, // 60 秒
			},
			"deploy": {
				Enabled: true,
				Timeout: 300 * 1000000000, // 5 分钟
			},
			"help": {
				Enabled: true,
				Timeout: 5 * 1000000000, // 5 秒
			},
			"version": {
				Enabled: true,
				Timeout: 5 * 1000000000, // 5 秒
			},
			"status": {
				Enabled: true,
				Timeout: 10 * 1000000000, // 10 秒
			},
			"reconnect": {
				Enabled: true,
				Timeout: 30 * 1000000000, // 30 秒
			},
		},
		Custom: make(map[string]domain.CustomCommandConfig),
		Settings: domain.VirtualCommandSettings{
			OnError: "continue",
			Output:  "color",
			History: false,
		},
	}
}

// mergeDefaults 合并默认值到配置中。
func (l *ConfigLoader) mergeDefaults(config *domain.VirtualCommandConfig) {
	defaults := l.getDefaultConfig()

	// 合并内置命令配置
	if config.Builtin == nil {
		config.Builtin = defaults.Builtin
	} else {
		for name, defaultCmd := range defaults.Builtin {
			if _, exists := config.Builtin[name]; !exists {
				config.Builtin[name] = defaultCmd
			}
		}
	}

	// 合并自定义命令配置
	if config.Custom == nil {
		config.Custom = make(map[string]domain.CustomCommandConfig)
	}

	// 合并设置
	if config.Settings.OnError == "" {
		config.Settings.OnError = defaults.Settings.OnError
	}
	if config.Settings.Output == "" {
		config.Settings.Output = defaults.Settings.Output
	}
}

// GetConfigPath 获取配置文件路径。
func (l *ConfigLoader) GetConfigPath() string {
	return l.filePath
}

// Reload 重新加载配置。
func (l *ConfigLoader) Reload() (*domain.VirtualCommandConfig, error) {
	l.mu.Lock()
	l.config = nil
	l.mu.Unlock()

	return l.Load()
}
