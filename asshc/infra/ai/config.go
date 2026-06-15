package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"assh/asshc/domain"
	"gopkg.in/yaml.v3"
)

// ConfigLoader 实现 AI 配置加载器。
// 配置文件位置：~/.assh/v2/ai.yml
type ConfigLoader struct {
	mu       sync.RWMutex
	config   *domain.AIConfig
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

	return filepath.Join(home, ".assh", "v2", "ai.yml")
}

// Load 加载 AI 配置。
func (l *ConfigLoader) Load() (*domain.AIConfig, error) {
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
	var config domain.AIConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 合并默认值
	l.mergeDefaults(&config)

	l.config = &config
	return l.config, nil
}

// Save 保存 AI 配置。
func (l *ConfigLoader) Save(config *domain.AIConfig) error {
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

// getDefaultConfig 获取默认配置。
func (l *ConfigLoader) getDefaultConfig() *domain.AIConfig {
	return &domain.AIConfig{
		Endpoint:    "https://api.openai.com/v1",
		Model:       "gpt-4",
		APIKey:      "",
		MaxTokens:   4096,
		Temperature: 0.3,
	}
}

// mergeDefaults 合并默认值到配置中。
func (l *ConfigLoader) mergeDefaults(config *domain.AIConfig) {
	defaults := l.getDefaultConfig()

	if config.Endpoint == "" {
		config.Endpoint = defaults.Endpoint
	}
	if config.Model == "" {
		config.Model = defaults.Model
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = defaults.MaxTokens
	}
	if config.Temperature == 0 {
		config.Temperature = defaults.Temperature
	}
}

// GetConfigPath 获取配置文件路径。
func (l *ConfigLoader) GetConfigPath() string {
	return l.filePath
}

// Reload 重新加载配置。
func (l *ConfigLoader) Reload() (*domain.AIConfig, error) {
	l.mu.Lock()
	l.config = nil
	l.mu.Unlock()

	return l.Load()
}
