package port

import (
	"assh/asshc/domain"
	"context"
)

// AIProvider 定义 AI 服务提供者的接口。
// 负责与 AI 服务（如 OpenAI、DeepSeek 等）进行交互。
type AIProvider interface {
	// Generate 根据用户提示生成内容。
	Generate(ctx context.Context, prompt string) (string, error)

	// GenerateWithSystem 使用系统提示和用户提示生成内容。
	GenerateWithSystem(ctx context.Context, system, user string) (string, error)

	// GenerateScript 根据漏洞信息生成检测脚本。
	GenerateScript(ctx context.Context, vulnInfo string, cveDetail *domain.CVEDetail) (string, error)

	// ValidateConnection 验证 AI 服务连接。
	ValidateConnection(ctx context.Context) error

	// GetModel 返回当前使用的模型名称。
	GetModel() string

	// SetModel 设置使用的模型名称。
	SetModel(model string)
}

// VulnerabilityChecker 定义漏洞检查器的接口。
// 负责漏洞检查的完整流程：输入验证 → CVE 查询 → AI 生成 → 安全审查 → 执行 → 报告。
type VulnerabilityChecker interface {
	// Check 执行完整的漏洞检查流程。
	Check(ctx context.Context, server *domain.Server, vulnInfo string) (*domain.VulnReport, error)

	// CheckBatch 批量检查多台服务器。
	CheckBatch(ctx context.Context, servers []*domain.Server, vulnInfo string) ([]*domain.VulnReport, error)

	// CheckGroup 检查指定分组的所有服务器。
	CheckGroup(ctx context.Context, group string, vulnInfo string) ([]*domain.VulnReport, error)

	// GenerateScript 生成检测脚本。
	GenerateScript(ctx context.Context, vulnInfo string, cveDetail *domain.CVEDetail) (string, error)

	// ValidateScript 验证脚本安全性。
	ValidateScript(script string) (*domain.ScriptValidation, error)

	// ExecuteScript 执行检测脚本。
	ExecuteScript(ctx context.Context, server *domain.Server, script string) (*domain.ExecResult, error)

	// SaveScript 保存脚本到本地。
	SaveScript(vulnID string, script string) (string, error)

	// SaveReport 保存报告到本地。
	SaveReport(vulnID string, report *domain.VulnReport) (string, error)
}

// CVEDetailFetcher 定义 CVE 详情获取器的接口。
// 负责从 NVD NIST API 获取 CVE 详细信息。
type CVEDetailFetcher interface {
	// Fetch 获取 CVE 详细信息。
	Fetch(ctx context.Context, cveID string) (*domain.CVEDetail, error)

	// FetchBatch 批量获取 CVE 详细信息。
	FetchBatch(ctx context.Context, cveIDs []string) (map[string]*domain.CVEDetail, error)

	// ValidateCVEID 验证 CVE ID 格式。
	ValidateCVEID(cveID string) bool
}

// SecurityScanner 定义安全扫描器的接口。
// 负责对生成的脚本进行安全审查。
type SecurityScanner interface {
	// CheckScript 检查脚本安全性。
	CheckScript(script string) (*domain.ScriptValidation, error)

	// CheckInput 检查用户输入安全性。
	CheckInput(input string) error

	// AddRule 添加安全规则。
	AddRule(rule *domain.SecurityRule) error

	// RemoveRule 移除安全规则。
	RemoveRule(name string) error

	// ListRules 列出所有安全规则。
	ListRules() []*domain.SecurityRule

	// LoadRules 加载安全规则。
	LoadRules(rules []domain.SecurityRule) error
}

// VulnStore 定义漏洞检查存储的接口。
// 负责存储和检索漏洞检查记录。
type VulnStore interface {
	// Save 保存漏洞检查记录。
	Save(check *domain.VulnCheck) error

	// Get 获取漏洞检查记录。
	Get(id string) (*domain.VulnCheck, error)

	// List 列出所有漏洞检查记录。
	List() ([]*domain.VulnCheck, error)

	// ListByTarget 列出指定目标的检查记录。
	ListByTarget(target string) ([]*domain.VulnCheck, error)

	// ListByVulnID 列出指定漏洞的检查记录。
	ListByVulnID(vulnID string) ([]*domain.VulnCheck, error)

	// Delete 删除漏洞检查记录。
	Delete(id string) error

	// Update 更新漏洞检查记录。
	Update(check *domain.VulnCheck) error
}
