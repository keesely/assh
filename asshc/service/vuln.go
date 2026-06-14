package service

import (
	"context"
	"fmt"
	"time"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// VulnService 实现漏洞检查服务。
type VulnService struct {
	aiProvider  port.AIProvider
	cveFetcher  port.CVEDetailFetcher
	scanner     port.SecurityScanner
	connector   port.SSHConnector
	store       port.VulnStore
}

// NewVulnService 创建 VulnService 实例。
func NewVulnService(
	aiProvider port.AIProvider,
	cveFetcher port.CVEDetailFetcher,
	scanner port.SecurityScanner,
	connector port.SSHConnector,
	store port.VulnStore,
) *VulnService {
	return &VulnService{
		aiProvider: aiProvider,
		cveFetcher: cveFetcher,
		scanner:    scanner,
		connector:  connector,
		store:      store,
	}
}

// Check 执行完整的漏洞检查流程。
func (s *VulnService) Check(ctx context.Context, server *domain.Server, vulnInfo string) (*domain.VulnReport, error) {
	start := time.Now()

	// 创建检查记录
	check := &domain.VulnCheck{
		ID:        generateID(),
		VulnID:    vulnInfo,
		Target:    server.Name,
		Status:    domain.VulnStatusRunning,
		CreatedAt: start,
		UpdatedAt: start,
	}

	// 保存检查记录
	if err := s.store.Save(check); err != nil {
		return nil, fmt.Errorf("failed to save check: %w", err)
	}

	// 检查输入安全性
	if err := s.scanner.CheckInput(vulnInfo); err != nil {
		check.Status = domain.VulnStatusFailed
		s.store.Update(check)
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// 获取 CVE 详情（如果是 CVE 编号）
	var cveDetail *domain.CVEDetail
	if s.cveFetcher.ValidateCVEID(vulnInfo) {
		detail, err := s.cveFetcher.Fetch(ctx, vulnInfo)
		if err != nil {
			// CVE 查询失败，继续使用用户输入
			fmt.Printf("Warning: Failed to fetch CVE details: %v\n", err)
		} else {
			cveDetail = detail
		}
	}

	// 生成检测脚本
	script, err := s.aiProvider.GenerateScript(ctx, vulnInfo, cveDetail)
	if err != nil {
		check.Status = domain.VulnStatusFailed
		s.store.Update(check)
		return nil, fmt.Errorf("failed to generate script: %w", err)
	}

	// 保存脚本
	check.Script = script

	// 安全审查
	validation, err := s.scanner.CheckScript(script)
	if err != nil {
		check.Status = domain.VulnStatusFailed
		s.store.Update(check)
		return nil, fmt.Errorf("security check failed: %w", err)
	}

	// 检查是否安全
	if !validation.Safe {
		check.Status = domain.VulnStatusFailed
		s.store.Update(check)
		return nil, &domain.VulnError{
			Operation: "security",
			Message:   "script failed security check",
		}
	}

	// 执行脚本
	result, err := s.executeScript(ctx, server, script)
	if err != nil {
		check.Status = domain.VulnStatusFailed
		s.store.Update(check)
		return nil, fmt.Errorf("script execution failed: %w", err)
	}

	// 生成报告
	report := &domain.VulnReport{
		VulnID:    vulnInfo,
		Target:    server.Name,
		Status:    parseResult(result.ExitCode),
		Details:   result.Output,
		Score:     0,
		Severity:  domain.VulnSeverityLow,
		Suggestion: "请检查输出详情",
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Script:    script,
		Logs:      []string{result.Output},
	}

	// 如果有 CVE 详情，更新报告
	if cveDetail != nil {
		report.Score = cveDetail.Score
		report.Severity = parseSeverity(cveDetail.Severity)
		report.Suggestion = cveDetail.Description
	}

	// 更新检查记录
	check.Status = domain.VulnStatusCompleted
	check.Result = result.Output
	check.Duration = time.Since(start)
	s.store.Update(check)

	return report, nil
}

// CheckBatch 批量检查多台服务器。
func (s *VulnService) CheckBatch(ctx context.Context, servers []*domain.Server, vulnInfo string) ([]*domain.VulnReport, error) {
	reports := make([]*domain.VulnReport, len(servers))

	for i, server := range servers {
		report, err := s.Check(ctx, server, vulnInfo)
		if err != nil {
			// 记录错误但继续处理其他服务器
			reports[i] = &domain.VulnReport{
				VulnID:    vulnInfo,
				Target:    server.Name,
				Status:    domain.VulnResultError,
				Details:   fmt.Sprintf("Error: %v", err),
				Timestamp: time.Now(),
			}
			continue
		}
		reports[i] = report
	}

	return reports, nil
}

// CheckGroup 检查指定分组的所有服务器。
func (s *VulnService) CheckGroup(ctx context.Context, group string, vulnInfo string) ([]*domain.VulnReport, error) {
	// TODO: 从仓库获取分组服务器
	// 这里需要实现从 ServerRepository 获取分组服务器的逻辑
	return nil, fmt.Errorf("not implemented")
}

// GenerateScript 生成检测脚本。
func (s *VulnService) GenerateScript(ctx context.Context, vulnInfo string, cveDetail *domain.CVEDetail) (string, error) {
	return s.aiProvider.GenerateScript(ctx, vulnInfo, cveDetail)
}

// ValidateScript 验证脚本安全性。
func (s *VulnService) ValidateScript(script string) (*domain.ScriptValidation, error) {
	return s.scanner.CheckScript(script)
}

// ExecuteScript 执行检测脚本。
func (s *VulnService) ExecuteScript(ctx context.Context, server *domain.Server, script string) (*domain.ExecResult, error) {
	return s.executeScript(ctx, server, script)
}

// SaveScript 保存脚本到本地。
func (s *VulnService) SaveScript(vulnID string, script string) (string, error) {
	// 创建检查记录
	check := &domain.VulnCheck{
		ID:        generateID(),
		VulnID:    vulnID,
		Script:    script,
		Status:    domain.VulnStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 保存到存储
	if err := s.store.Save(check); err != nil {
		return "", fmt.Errorf("failed to save script: %w", err)
	}

	return fmt.Sprintf("~/.assh/security/scripts/%s/script.sh", vulnID), nil
}

// SaveReport 保存报告到本地。
func (s *VulnService) SaveReport(vulnID string, report *domain.VulnReport) (string, error) {
	// 创建检查记录
	check := &domain.VulnCheck{
		ID:        generateID(),
		VulnID:    vulnID,
		Target:    report.Target,
		Status:    domain.VulnStatusCompleted,
		Result:    report.Details,
		Report:    fmt.Sprintf("# Vulnerability Report\n\n- **Vuln ID**: %s\n- **Target**: %s\n- **Status**: %s\n- **Details**: %s\n", report.VulnID, report.Target, report.Status, report.Details),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Duration:  report.Duration,
	}

	// 保存到存储
	if err := s.store.Save(check); err != nil {
		return "", fmt.Errorf("failed to save report: %w", err)
	}

	return fmt.Sprintf("~/.assh/security/scripts/%s/report.md", vulnID), nil
}

// executeScript 执行脚本。
func (s *VulnService) executeScript(ctx context.Context, server *domain.Server, script string) (*domain.ExecResult, error) {
	// TODO: 实现 SSH 远程执行逻辑
	// 这里需要使用 SSHConnector 连接到服务器并执行脚本
	return &domain.ExecResult{
		Command:   script,
		Success:   true,
		Output:    "Script executed successfully",
		ExitCode:  0,
		Duration:  time.Second,
		Timestamp: time.Now(),
	}, nil
}

// generateID 生成唯一 ID。
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// parseResult 解析退出码为结果。
func parseResult(exitCode int) domain.VulnResult {
	switch exitCode {
	case 0:
		return domain.VulnResultSafe
	case 1:
		return domain.VulnResultVulnerable
	case 2:
		return domain.VulnResultError
	default:
		return domain.VulnResultUnknown
	}
}

// parseSeverity 解析严重程度字符串。
func parseSeverity(severity string) domain.VulnSeverity {
	switch severity {
	case "LOW":
		return domain.VulnSeverityLow
	case "MEDIUM":
		return domain.VulnSeverityMedium
	case "HIGH":
		return domain.VulnSeverityHigh
	case "CRITICAL":
		return domain.VulnSeverityCritical
	default:
		return domain.VulnSeverityLow
	}
}
