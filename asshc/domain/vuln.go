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

// VulnCheck 表示一次漏洞检查的完整信息。
type VulnCheck struct {
	ID        string        `json:"id"`        // 检查 ID
	VulnID    string        `json:"vulnId"`    // 漏洞 ID（如 CVE-2024-3094）
	Target    string        `json:"target"`    // 目标服务器
	Status    VulnStatus    `json:"status"`    // 检查状态
	Script    string        `json:"script"`    // 生成的脚本
	Result    string        `json:"result"`    // 执行结果
	Report    string        `json:"report"`    // Markdown 报告
	CreatedAt time.Time     `json:"createdAt"` // 创建时间
	UpdatedAt time.Time     `json:"updatedAt"` // 更新时间
	Duration  time.Duration `json:"duration"`  // 执行时长
}

// VulnStatus 表示漏洞检查的状态。
type VulnStatus int

const (
	VulnStatusPending   VulnStatus = iota // 待执行
	VulnStatusRunning                     // 执行中
	VulnStatusCompleted                   // 已完成
	VulnStatusFailed                      // 失败
	VulnStatusCancelled                   // 已取消
)

// String 返回状态的字符串表示。
func (s VulnStatus) String() string {
	switch s {
	case VulnStatusPending:
		return "pending"
	case VulnStatusRunning:
		return "running"
	case VulnStatusCompleted:
		return "completed"
	case VulnStatusFailed:
		return "failed"
	case VulnStatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// VulnReport 表示漏洞检查报告。
type VulnReport struct {
	VulnID    string        `json:"vulnId"`    // 漏洞 ID
	Target    string        `json:"target"`    // 目标服务器
	Status    VulnResult    `json:"status"`    // 检查结果
	Details   string        `json:"details"`   // 详细信息
	Score     float64       `json:"score"`     // 风险评分
	Severity  VulnSeverity  `json:"severity"`  // 严重程度
	Suggestion string       `json:"suggestion"` // 修复建议
	Timestamp time.Time     `json:"timestamp"` // 检查时间
	Duration  time.Duration `json:"duration"`  // 执行时长
	Script    string        `json:"script"`    // 执行的脚本
	Logs      []string      `json:"logs"`      // 执行日志
}

// VulnResult 表示漏洞检查结果。
type VulnResult int

const (
	VulnResultSafe      VulnResult = iota // 安全
	VulnResultVulnerable                  // 存在漏洞
	VulnResultUnknown                     // 未知
	VulnResultError                       // 错误
)

// String 返回结果的字符串表示。
func (r VulnResult) String() string {
	switch r {
	case VulnResultSafe:
		return "safe"
	case VulnResultVulnerable:
		return "vulnerable"
	case VulnResultUnknown:
		return "unknown"
	case VulnResultError:
		return "error"
	default:
		return "unknown"
	}
}

// VulnSeverity 表示漏洞严重程度。
type VulnSeverity int

const (
	VulnSeverityLow      VulnSeverity = iota // 低
	VulnSeverityMedium                       // 中
	VulnSeverityHigh                         // 高
	VulnSeverityCritical                     // 严重
)

// String 返回严重程度的字符串表示。
func (s VulnSeverity) String() string {
	switch s {
	case VulnSeverityLow:
		return "low"
	case VulnSeverityMedium:
		return "medium"
	case VulnSeverityHigh:
		return "high"
	case VulnSeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// CVEDetail 表示 CVE 详细信息。
type CVEDetail struct {
	ID          string    `json:"id"`          // CVE ID
	Description string    `json:"description"` // 描述
	Score       float64   `json:"score"`       // CVSS 评分
	Severity    string    `json:"severity"`    // 严重程度
	Published   time.Time `json:"published"`   // 发布时间
	Modified    time.Time `json:"modified"`    // 修改时间
	References  []string  `json:"references"`  // 参考链接
}

// ScriptValidation 表示脚本安全审查结果。
type ScriptValidation struct {
	Safe         bool       `json:"safe"`         // 是否安全
	RiskLevel    RiskLevel  `json:"riskLevel"`    // 风险级别
	Warnings     []string   `json:"warnings"`     // 警告信息
	MatchedRules []string   `json:"matchedRules"` // 匹配的规则
	Suggestions  []string   `json:"suggestions"`  // 建议
}

// RiskLevel 表示风险级别。
type RiskLevel int

const (
	RiskLow    RiskLevel = iota // 低风险
	RiskMedium                  // 中风险
	RiskHigh                    // 高风险
)

// String 返回风险级别的字符串表示。
func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	default:
		return "unknown"
	}
}

// SecurityRule 表示安全审查规则。
type SecurityRule struct {
	Name        string   `json:"name"`        // 规则名称
	Pattern     string   `json:"pattern"`     // 匹配模式
	RiskLevel   RiskLevel `json:"riskLevel"`   // 风险级别
	Message     string   `json:"message"`     // 警告信息
	Enabled     bool     `json:"enabled"`     // 是否启用
	Custom      bool     `json:"custom"`      // 是否自定义规则
}

// ExecResult 表示命令执行结果。
type ExecResult struct {
	Command   string        `json:"command"`   // 执行的命令
	Success   bool          `json:"success"`   // 是否成功
	Output    string        `json:"output"`    // 输出内容
	Error     string        `json:"error"`     // 错误信息
	ExitCode  int           `json:"exitCode"`  // 退出码
	Duration  time.Duration `json:"duration"`  // 执行时长
	Timestamp time.Time     `json:"timestamp"` // 执行时间
}

// AIConfig 表示 AI 服务配置。
type AIConfig struct {
	Endpoint    string  `yaml:"endpoint"`    // API 地址
	Model       string  `yaml:"model"`       // 模型名称
	APIKey      string  `yaml:"api_key"`     // API 密钥
	MaxTokens   int     `yaml:"max_tokens"`  // 最大 Token 数
	Temperature float64 `yaml:"temperature"` // 温度参数
}

// SecurityConfig 表示安全配置。
type SecurityConfig struct {
	Rules       []SecurityRule `yaml:"rules"`       // 内置规则
	CustomRules []SecurityRule `yaml:"customRules"` // 自定义规则
	ConfirmHigh bool           `yaml:"confirmHigh"` // 高风险操作确认
	Timeout     time.Duration  `yaml:"timeout"`     // 执行超时
}

// VulnError 表示漏洞检查相关的错误。
type VulnError struct {
	Operation string
	Message   string
	Err       error
}

func (e *VulnError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("vuln %s: %s: %v", e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("vuln %s: %s", e.Operation, e.Message)
}

// 预定义的漏洞检查错误。
var (
	ErrVulnNotFound     = &VulnError{Operation: "check", Message: "vulnerability not found"}
	ErrVulnExists       = &VulnError{Operation: "check", Message: "vulnerability already exists"}
	ErrInvalidCVE       = &VulnError{Operation: "input", Message: "invalid CVE format"}
	ErrAIRateLimit      = &VulnError{Operation: "ai", Message: "AI service rate limit exceeded"}
	ErrAIServiceDown    = &VulnError{Operation: "ai", Message: "AI service unavailable"}
	ErrScriptUnsafe     = &VulnError{Operation: "security", Message: "script failed security check"}
	ErrExecutionFailed  = &VulnError{Operation: "execute", Message: "script execution failed"}
	ErrTimeout          = &VulnError{Operation: "execute", Message: "execution timeout"}
)
