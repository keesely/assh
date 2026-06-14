package security

import (
	"fmt"
	"regexp"
	"sync"

	"assh/asshc/domain"
)

// Scanner 实现安全扫描器。
type Scanner struct {
	mu    sync.RWMutex
	rules []*domain.SecurityRule
}

// NewScanner 创建 Scanner 实例。
func NewScanner() *Scanner {
	scanner := &Scanner{}
	scanner.loadBuiltinRules()
	return scanner
}

// CheckScript 检查脚本安全性。
func (s *Scanner) CheckScript(script string) (*domain.ScriptValidation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	validation := &domain.ScriptValidation{
		Safe:         true,
		RiskLevel:    domain.RiskLow,
		Warnings:     []string{},
		MatchedRules: []string{},
		Suggestions:  []string{},
	}

	// 检查所有规则
	for _, rule := range s.rules {
		if !rule.Enabled {
			continue
		}

		matched, err := regexp.MatchString(rule.Pattern, script)
		if err != nil {
			continue
		}

		if matched {
			validation.Safe = false
			validation.MatchedRules = append(validation.MatchedRules, rule.Name)
			validation.Warnings = append(validation.Warnings, rule.Message)

			// 更新风险级别
			if rule.RiskLevel > validation.RiskLevel {
				validation.RiskLevel = rule.RiskLevel
			}
		}
	}

	// 添加建议
	if !validation.Safe {
		validation.Suggestions = append(validation.Suggestions, "请审查脚本内容，确保没有危险操作")
		if validation.RiskLevel >= domain.RiskHigh {
			validation.Suggestions = append(validation.Suggestions, "建议在执行前进行人工审查")
		}
	}

	return validation, nil
}

// CheckInput 检查用户输入安全性。
func (s *Scanner) CheckInput(input string) error {
	// 检查输入长度
	if len(input) > 10000 {
		return fmt.Errorf("input too long: %d characters", len(input))
	}

	// 检查特殊字符
	dangerousPatterns := []string{
		`<script>`,
		`javascript:`,
		`on\w+\s*=`,
	}

	for _, pattern := range dangerousPatterns {
		matched, _ := regexp.MatchString(pattern, input)
		if matched {
			return fmt.Errorf("potentially dangerous input detected")
		}
	}

	return nil
}

// GetRules 获取所有安全规则。
func (s *Scanner) GetRules() []*domain.SecurityRule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rules := make([]*domain.SecurityRule, len(s.rules))
	copy(rules, s.rules)
	return rules
}

// AddRule 添加安全规则。
func (s *Scanner) AddRule(rule *domain.SecurityRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查规则是否已存在
	for _, r := range s.rules {
		if r.Name == rule.Name {
			return fmt.Errorf("rule already exists: %s", rule.Name)
		}
	}

	s.rules = append(s.rules, rule)
	return nil
}

// RemoveRule 移除安全规则。
func (s *Scanner) RemoveRule(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, rule := range s.rules {
		if rule.Name == name {
			s.rules = append(s.rules[:i], s.rules[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("rule not found: %s", name)
}

// ListRules 列出所有安全规则。
func (s *Scanner) ListRules() []*domain.SecurityRule {
	return s.GetRules()
}

// LoadRules 加载安全规则。
func (s *Scanner) LoadRules(rules []domain.SecurityRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rules = make([]*domain.SecurityRule, len(rules))
	for i := range rules {
		s.rules[i] = &rules[i]
	}

	return nil
}

// loadBuiltinRules 加载内置规则。
func (s *Scanner) loadBuiltinRules() {
	builtinRules := []domain.SecurityRule{
		{
			Name:      "禁止删除根目录",
			Pattern:   `rm\s+-rf\s+/`,
			RiskLevel: domain.RiskHigh,
			Message:   "禁止删除根目录",
			Enabled:   true,
			Custom:    false,
		},
		{
			Name:      "禁止格式化操作",
			Pattern:   `mkfs|dd\s+if=|format`,
			RiskLevel: domain.RiskHigh,
			Message:   "禁止格式化操作",
			Enabled:   true,
			Custom:    false,
		},
		{
			Name:      "禁止 777 权限",
			Pattern:   `chmod\s+777`,
			RiskLevel: domain.RiskMedium,
			Message:   "禁止设置 777 权限",
			Enabled:   true,
			Custom:    false,
		},
		{
			Name:      "禁止直接写块设备",
			Pattern:   `>/dev/(sda|sdb|nvme)`,
			RiskLevel: domain.RiskHigh,
			Message:   "禁止直接写入块设备",
			Enabled:   true,
			Custom:    false,
		},
		{
			Name:      "禁止管道执行下载内容",
			Pattern:   `wget|curl.*\|\s*sh`,
			RiskLevel: domain.RiskHigh,
			Message:   "禁止管道执行下载内容",
			Enabled:   true,
			Custom:    false,
		},
		{
			Name:      "禁止 fork 炸弹",
			Pattern:   `:\(\)\{\s*:\|:&\s*\};:`,
			RiskLevel: domain.RiskHigh,
			Message:   "禁止 fork 炸弹",
			Enabled:   true,
			Custom:    false,
		},
		{
			Name:      "禁止清空防火墙规则",
			Pattern:   `iptables.*-F`,
			RiskLevel: domain.RiskHigh,
			Message:   "禁止清空防火墙规则",
			Enabled:   true,
			Custom:    false,
		},
		{
			Name:      "禁止修改密码",
			Pattern:   `passwd`,
			RiskLevel: domain.RiskMedium,
			Message:   "禁止修改密码",
			Enabled:   true,
			Custom:    false,
		},
		{
			Name:      "服务操作",
			Pattern:   `systemctl\s+(stop|restart|disable)`,
			RiskLevel: domain.RiskMedium,
			Message:   "服务操作需要确认",
			Enabled:   true,
			Custom:    false,
		},
		{
			Name:      "用户管理",
			Pattern:   `useradd|userdel|groupadd`,
			RiskLevel: domain.RiskMedium,
			Message:   "用户管理操作需要确认",
			Enabled:   true,
			Custom:    false,
		},
		{
			Name:      "定时任务",
			Pattern:   `crontab\s+-e`,
			RiskLevel: domain.RiskMedium,
			Message:   "定时任务操作需要确认",
			Enabled:   true,
			Custom:    false,
		},
		{
			Name:      "挂载操作",
			Pattern:   `mount|umount`,
			RiskLevel: domain.RiskLow,
			Message:   "挂载操作需要确认",
			Enabled:   true,
			Custom:    false,
		},
	}

	s.rules = make([]*domain.SecurityRule, len(builtinRules))
	for i := range builtinRules {
		s.rules[i] = &builtinRules[i]
	}
}
