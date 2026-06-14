package port

import (
	"assh/asshc/domain"
)

// SecurityChecker 定义安全检查器的接口。
// 负责对 AI 生成的脚本进行安全审查。
type SecurityChecker interface {
	// CheckScript 检查脚本安全性。
	CheckScript(script string) (*domain.ScriptValidation, error)

	// CheckInput 检查用户输入安全性。
	CheckInput(input string) error

	// GetRules 获取所有安全规则。
	GetRules() []*domain.SecurityRule

	// AddRule 添加安全规则。
	AddRule(rule *domain.SecurityRule) error

	// RemoveRule 移除安全规则。
	RemoveRule(name string) error

	// LoadRules 加载安全规则配置。
	LoadRules(config *domain.SecurityConfig) error
}

// SecurityConfigLoader 定义安全配置加载器的接口。
type SecurityConfigLoader interface {
	// Load 加载安全配置。
	Load() (*domain.SecurityConfig, error)

	// Save 保存安全配置。
	Save(config *domain.SecurityConfig) error

	// GetRules 获取安全规则。
	GetRules() ([]domain.SecurityRule, error)

	// AddRule 添加安全规则。
	AddRule(rule domain.SecurityRule) error

	// RemoveRule 移除安全规则。
	RemoveRule(name string) error
}
