package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"assh/asshc/domain"
	"assh/asshc/port"
	"golang.org/x/crypto/ssh"
)

// StepLogger 提供结构化的步骤日志输出，统一前缀和格式。
type StepLogger struct {
	prefix string
}

func newStepLogger(serverName string) *StepLogger {
	return &StepLogger{prefix: fmt.Sprintf("[%s]", serverName)}
}

func (l *StepLogger) Log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "  %s %s\n", l.prefix, msg)
}

func (l *StepLogger) Section(title string) {
	fmt.Fprintf(os.Stderr, "\n── %s ─────────────────────────────\n", title)
}

func (l *StepLogger) Done() {
	fmt.Fprintf(os.Stderr, "  %s ✅ Done\n", l.prefix)
}

// VulnService 实现漏洞检查服务。
type VulnService struct {
	aiProvider  port.AIProvider
	cveFetcher  port.CVEDetailFetcher
	scanner     port.SecurityScanner
	connector   port.SSHConnector
	store       port.VulnStore
	showLogs    bool // --show-logs 启用时输出 HTTP 请求/响应详情
}

// SetShowLogs 启用或禁用 HTTP 请求/响应调试输出。
// 当启用时，AI provider 和 CVE fetcher 的 HTTP 请求/响应详情会输出到 stderr。
func (s *VulnService) SetShowLogs(on bool) {
	s.showLogs = on
	// 通过类型断言将调试模式传递给具体实现
	if ai, ok := s.aiProvider.(interface{ SetDebug(bool) }); ok {
		ai.SetDebug(on)
	}
	if cve, ok := s.cveFetcher.(interface{ SetDebug(bool) }); ok {
		cve.SetDebug(on)
	}
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
	log := newStepLogger(server.Name)
	logs := []string{}

	log.Section("AI 安全检查")
	log.Log("🔍 目标: %s  漏洞: %s", server.Name, vulnInfo)

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
	logs = append(logs, fmt.Sprintf("[%.3fs] Check record created", time.Since(start).Seconds()))

	// 检查输入安全性
	log.Log("Step 1/5: 输入安全检查 ...")
	if err := s.scanner.CheckInput(vulnInfo); err != nil {
		check.Status = domain.VulnStatusFailed
		s.store.Update(check)
		return nil, fmt.Errorf("input validation failed: %w", err)
	}
	log.Log("✅ 输入安全")
	logs = append(logs, fmt.Sprintf("[%.3fs] Input validation passed", time.Since(start).Seconds()))

	// 获取 CVE 详情（如果是 CVE 编号）
	log.Log("Step 2/5: 查询 CVE 详情 ...")
	var cveDetail *domain.CVEDetail
	if s.cveFetcher.ValidateCVEID(vulnInfo) {
		detail, err := s.cveFetcher.Fetch(ctx, vulnInfo)
		if err != nil {
			log.Log("⚠ CVE 查询失败: %v (继续使用原始输入)", err)
			logs = append(logs, fmt.Sprintf("[%.3fs] CVE lookup failed: %v", time.Since(start).Seconds(), err))
		} else {
			cveDetail = detail
			log.Log("✅ CVE 详情: 评分 %.1f, 严重程度 %s", cveDetail.Score, cveDetail.Severity)
			logs = append(logs, fmt.Sprintf("[%.3fs] CVE detail: score=%.1f severity=%s",
				time.Since(start).Seconds(), cveDetail.Score, cveDetail.Severity))
		}
	} else {
		log.Log("⏭ 非 CVE 格式，跳过查询")
	}

	// 生成检测脚本
	log.Log("Step 3/5: AI 生成检测脚本 ...")
	script, err := s.aiProvider.GenerateScript(ctx, vulnInfo, cveDetail)
	if err != nil {
		check.Status = domain.VulnStatusFailed
		s.store.Update(check)
		return nil, fmt.Errorf("failed to generate script: %w", err)
	}
	log.Log("✅ 脚本已生成 (%d 行)", strings.Count(script, "\n")+1)
	logs = append(logs, fmt.Sprintf("[%.3fs] Script generated (%d bytes)", time.Since(start).Seconds(), len(script)))

	// 保存脚本
	check.Script = script

	// 安全审查
	log.Log("Step 4/5: 安全审查 ...")
	validation, err := s.scanner.CheckScript(script)
	if err != nil {
		check.Status = domain.VulnStatusFailed
		s.store.Update(check)
		return nil, fmt.Errorf("security check failed: %w", err)
	}
	if validation.Safe {
		if len(validation.Warnings) > 0 {
			log.Log("✅ 安全审查通过 (%d 个警告)", len(validation.Warnings))
		} else {
			log.Log("✅ 安全审查通过")
		}
	} else {
		log.Log("❌ 安全审查未通过: %v", validation.MatchedRules)
		check.Status = domain.VulnStatusFailed
		s.store.Update(check)
		return nil, &domain.VulnError{
			Operation: "security",
			Message:   "script failed security check",
		}
	}
	logs = append(logs, fmt.Sprintf("[%.3fs] Security review: safe=%v warnings=%d",
		time.Since(start).Seconds(), validation.Safe, len(validation.Warnings)))

	// Step 5: 执行脚本
	log.Log("Step 5/5: 远程执行检测脚本 ...")
	log.Log("服务器: %s (%s)", server.Name, server.Host)
	result, err := s.executeScript(ctx, server, script)
	if err != nil {
		check.Status = domain.VulnStatusFailed
		s.store.Update(check)
		return nil, fmt.Errorf("script execution failed: %w", err)
	}

	resultStatus := parseResult(result.ExitCode)
	log.Log("✅ 执行完成 (退出码: %d, 耗时: %s)", result.ExitCode, result.Duration.Round(time.Millisecond))
	logs = append(logs, fmt.Sprintf("[%.3fs] Execution complete: exitCode=%d duration=%s output=%d bytes",
		time.Since(start).Seconds(), result.ExitCode, result.Duration, len(result.Output)))

	// 输出脚本日志
	log.Section("执行日志")
	log.Log("退出码: %d → %s", result.ExitCode, resultStatus.String())
	log.Log("执行时长: %s", result.Duration.Round(time.Millisecond))
	if result.Output != "" {
		log.Log("脚本输出 (%d bytes):", len(result.Output))
		for _, line := range strings.Split(strings.TrimRight(result.Output, "\n"), "\n") {
			fmt.Fprintf(os.Stderr, "    | %s\n", line)
		}
	}
	logs = append(logs, fmt.Sprintf("[%.3fs] Output:\n%s", time.Since(start).Seconds(), result.Output))
	logs = append(logs, fmt.Sprintf("[%.3fs] Script:\n%s", time.Since(start).Seconds(), script))

	// AI 分析执行结果
	log.Section("AI 分析")
	log.Log("正在分析执行结果 ...")
	analysisCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	analysis, err := s.aiProvider.AnalyzeExecution(analysisCtx, vulnInfo, script, result.Output, result.ExitCode)
	if err != nil {
		log.Log("⚠ AI 分析失败: %v", err)
		analysis = fmt.Sprintf("AI 分析暂不可用。服务器 %s 的检查已完成，退出码 %d。",
			server.Name, result.ExitCode)
		logs = append(logs, fmt.Sprintf("[%.3fs] AI analysis failed: %v", time.Since(start).Seconds(), err))
	} else {
		log.Log("✅ 分析完成")
		logs = append(logs, fmt.Sprintf("[%.3fs] AI analysis:\n%s", time.Since(start).Seconds(), analysis))
	}

	// 生成报告
	report := &domain.VulnReport{
		VulnID:     vulnInfo,
		Target:     server.Name,
		Status:     resultStatus,
		Details:    result.Output,
		Score:      0,
		Severity:   domain.VulnSeverityLow,
		Suggestion: "请检查 AI 分析结果",
		Timestamp:  time.Now(),
		Duration:   time.Since(start),
		Script:     script,
		Logs:       logs,
		AIAnalysis: analysis,
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

	log.Done()

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

// executeScript 通过 SSH 在远程服务器上执行脚本。
// 使用 heredoc 管道写入临时文件 → 执行 → 清理，避免 escaping 问题。
// 所有执行细节输出到 stderr，让用户了解每一步。
func (s *VulnService) executeScript(ctx context.Context, server *domain.Server, script string) (*domain.ExecResult, error) {
	start := time.Now()

	fmt.Fprintf(os.Stderr, "  ┌─ SSH 连接 ──────────────────────────\n")
	fmt.Fprintf(os.Stderr, "  │ 服务器: %s (%s)\n", server.Name, server.Host)

	client, err := s.connector.Connect(server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  │ ❌ 连接失败: %v\n", err)
		fmt.Fprintf(os.Stderr, "  └──────────────────────────────────────\n")
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer s.connector.Close(client)

	fmt.Fprintf(os.Stderr, "  │ ✅ 连接成功\n")
	fmt.Fprintf(os.Stderr, "  └──────────────────────────────────────\n")

	// 使用 heredoc 将脚本写入临时文件 → 执行 → 清理
	// 避免 bash -c 的 escaping 问题，支持任意多行脚本
	cmd := fmt.Sprintf(`SCRIPT=$(mktemp) && cat > "$SCRIPT" << 'ASSHEOF'
%s
ASSHEOF
chmod +x "$SCRIPT" && "$SCRIPT"; exitcode=$?; rm -f "$SCRIPT"; exit $exitcode`, script)

	fmt.Fprintf(os.Stderr, "  ┌─ 远程执行 ──────────────────────────\n")
	fmt.Fprintf(os.Stderr, "  │ 脚本将通过 heredoc 写入临时文件并执行\n")
	// 显示脚本内容
	for _, line := range strings.Split(script, "\n") {
		fmt.Fprintf(os.Stderr, "  │ %s\n", line)
	}

	session, err := client.NewSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  │ ❌ 会话创建失败: %v\n", err)
		fmt.Fprintf(os.Stderr, "  └──────────────────────────────────────\n")
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	fmt.Fprintf(os.Stderr, "  │ 执行中 ...\n")

	output, err := session.CombinedOutput(cmd)
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			fmt.Fprintf(os.Stderr, "  │ ❌ 执行失败: %v\n", err)
			fmt.Fprintf(os.Stderr, "  └──────────────────────────────────────\n")
			return nil, fmt.Errorf("script execution failed: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "  │ ✅ 执行完成 (退出码: %d)\n", exitCode)
	fmt.Fprintf(os.Stderr, "  └──────────────────────────────────────\n")

	return &domain.ExecResult{
		Command:   script,
		Success:   exitCode == 0,
		Output:    string(output),
		ExitCode:  exitCode,
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}, nil
}

// ListRules 列出所有安全规则。
func (s *VulnService) ListRules() []*domain.SecurityRule {
	return s.scanner.ListRules()
}

// AddRule 添加安全规则。
func (s *VulnService) AddRule(rule *domain.SecurityRule) error {
	return s.scanner.AddRule(rule)
}

// RemoveRule 移除安全规则。
func (s *VulnService) RemoveRule(name string) error {
	return s.scanner.RemoveRule(name)
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
