package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"assh/asshc/domain"
	"github.com/urfave/cli"
)

// registerSecurityCommands 注册安全检查命令。
// 通过闭包捕获 App 实例，访问 vulnSvc 和 serverSvc 执行实际检查。
func (a *App) registerSecurityCommands() {
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:  "security",
		Usage: "Security vulnerability checking",
		Subcommands: []cli.Command{
			{
				Name:  "vuln-check",
				Usage: "Check vulnerability using AI",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "server, s",
						Usage: "Target server (comma separated)",
					},
					cli.StringFlag{
						Name:  "group, g",
						Usage: "Target server group",
					},
					cli.StringFlag{
						Name:  "output, o",
						Usage: "Output format (text/json/markdown)",
						Value: "text",
					},
					cli.BoolFlag{
						Name:  "no-execute",
						Usage: "Only generate script, don't execute",
					},
					cli.BoolFlag{
						Name:  "no-confirm",
						Usage: "Skip high-risk operation confirmation",
					},
					cli.DurationFlag{
						Name:  "timeout",
						Usage: "Execution timeout",
						Value: 300 * time.Second,
					},
					cli.BoolFlag{
						Name:  "show-logs",
						Usage: "Show full HTTP request/response details (debug)",
					},
				},
				Action: func(c *cli.Context) error {
					server := c.String("server")
					group := c.String("group")
					vulnInfo := c.Args().First()
					format := c.String("output")
					noExec := c.Bool("no-execute")
					execTimeout := c.Duration("timeout")
					showLogs := c.Bool("show-logs")

					if vulnInfo == "" {
						return fmt.Errorf("vulnerability information is required")
					}
					if server == "" && group == "" {
						return fmt.Errorf("server or group is required")
					}
					if a.vulnSvc == nil {
						return fmt.Errorf("vulnerability check service not available")
					}

					// 启用 HTTP 请求/响应调试（输出到 stderr）
					a.vulnSvc.SetShowLogs(showLogs)

					ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
					defer cancel()

					if server != "" {
						servers := strings.Split(server, ",")
						for _, s := range servers {
							s = strings.TrimSpace(s)
							if s == "" {
								continue
							}
							if err := a.runVulnCheck(ctx, s, vulnInfo, format, noExec, showLogs); err != nil {
								fmt.Fprintf(c.App.Writer, "Error checking %s: %v\n", s, err)
							}
						}
					} else if group != "" {
						fmt.Fprintf(c.App.Writer, "Group check not yet implemented\n")
					}
					return nil
				},
			},
			{
				Name:  "rules",
				Usage: "Manage security rules",
				Subcommands: []cli.Command{
					{
						Name:  "list",
						Usage: "List all security rules",
						Action: func(c *cli.Context) error {
							if a.vulnSvc == nil {
								return fmt.Errorf("vulnerability check service not available")
							}
							rules := a.vulnSvc.ListRules()
							fmt.Fprintln(c.App.Writer, "Security Rules:")
							fmt.Fprintln(c.App.Writer, "─────────────────────────────────")
							if len(rules) == 0 {
								fmt.Fprintln(c.App.Writer, "  (no rules)")
							}
							for _, r := range rules {
								status := "✅"
								if !r.Enabled {
									status = "⛔"
								}
								fmt.Fprintf(c.App.Writer, "  %s %-30s [%s] %s\n",
									status, r.Name, r.RiskLevel, r.Message)
							}
							return nil
						},
					},
					{
						Name:  "add",
						Usage: "Add a security rule: <name> <regex-pattern>",
						Action: func(c *cli.Context) error {
							if a.vulnSvc == nil {
								return fmt.Errorf("vulnerability check service not available")
							}
							name := c.Args().First()
							if name == "" {
								return fmt.Errorf("rule name is required")
							}
							pattern := c.Args().Get(1)
							if pattern == "" {
								return fmt.Errorf("rule regex pattern is required")
							}
							rule := &domain.SecurityRule{
								Name:      name,
								Pattern:   pattern,
								RiskLevel: domain.RiskMedium,
								Message:   "Custom rule: " + name,
								Enabled:   true,
								Custom:    true,
							}
							if err := a.vulnSvc.AddRule(rule); err != nil {
								return fmt.Errorf("failed to add rule: %w", err)
							}
							fmt.Fprintf(c.App.Writer, "Rule added: %s\n", name)
							return nil
						},
					},
					{
						Name:  "remove",
						Usage: "Remove a security rule: <name>",
						Action: func(c *cli.Context) error {
							if a.vulnSvc == nil {
								return fmt.Errorf("vulnerability check service not available")
							}
							name := c.Args().First()
							if name == "" {
								return fmt.Errorf("rule name is required")
							}
							if err := a.vulnSvc.RemoveRule(name); err != nil {
								return fmt.Errorf("failed to remove rule: %w", err)
							}
							fmt.Fprintf(c.App.Writer, "Rule removed: %s\n", name)
							return nil
						},
					},
				},
			},
		},
	})
}

// runVulnCheck 对单台服务器执行漏洞检查并输出报告。
func (a *App) runVulnCheck(ctx context.Context, serverName, vulnInfo, format string, noExec, showLogs bool) error {
	svr, err := a.serverSvc.GetServer(serverName)
	if err != nil {
		return fmt.Errorf("server '%s' not found: %w", serverName, err)
	}

	if noExec {
		fmt.Fprintf(os.Stderr, "🔍 服务器: %s  漏洞: %s (仅生成脚本)\n", serverName, vulnInfo)
		script, err := a.vulnSvc.GenerateScript(ctx, vulnInfo, nil)
		if err != nil {
			return fmt.Errorf("failed to generate script: %w", err)
		}
		fmt.Fprintln(os.Stderr, "── 生成的检测脚本 ────────────────────")
		fmt.Fprintln(os.Stderr, script)
		fmt.Fprintln(os.Stderr, "──────────────────────────────────────")
		fmt.Fprintln(os.Stderr, "(未执行，--no-execute 指定)")
		return nil
	}

	// VulnService.Check() 内部已输出详细执行日志到 stderr
	report, err := a.vulnSvc.Check(ctx, svr, vulnInfo)
	if err != nil {
		return fmt.Errorf("check failed: %w", err)
	}

	outputReport(report, format)
	return nil
}

// outputReport 输出检查报告。
func outputReport(report *domain.VulnReport, format string) {
	switch format {
	case "json":
		outputJSON(report)
	case "markdown":
		outputMarkdown(report)
	default:
		outputText(report)
	}
}

// outputText 输出文本格式。
func outputText(report *domain.VulnReport) {
	fmt.Println("\n═══════════════════════════════════════════════════════════")
	fmt.Printf("  Vulnerability Check Report\n")
	fmt.Printf("  Vulnerability: %s\n", report.VulnID)
	fmt.Printf("  Target:        %s\n", report.Target)
	fmt.Printf("  Timestamp:     %s\n", report.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Duration:      %s\n", report.Duration.Round(time.Millisecond))
	fmt.Println("─────────────────────────────────────────────────────────")
	if report.Score > 0 {
		fmt.Printf("  CVSS Score:    %.1f (%s)\n", report.Score, report.Severity)
	}
	fmt.Printf("  Result:        %s\n", getStatusText(report.Status))
	if report.Details != "" {
		fmt.Println("── Script Execution Output ──────────────")
		fmt.Println(report.Details)
	}
	if report.AIAnalysis != "" {
		fmt.Println("── AI Analysis ───────────────────────────")
		fmt.Println(report.AIAnalysis)
	}
	fmt.Println("═══════════════════════════════════════════════════════════")
}

// outputJSON 输出 JSON 格式。
func outputJSON(report *domain.VulnReport) {
	fmt.Printf("JSON output not implemented yet\n")
}

// outputMarkdown 输出 Markdown 格式。
func outputMarkdown(report *domain.VulnReport) {
	fmt.Printf("# Vulnerability Check Report\n\n")
	fmt.Printf("| Field | Value |\n")
	fmt.Printf("|-------|-------|\n")
	fmt.Printf("| Vulnerability | %s |\n", report.VulnID)
	fmt.Printf("| Target | %s |\n", report.Target)
	fmt.Printf("| Timestamp | %s |\n", report.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("| Duration | %s |\n", report.Duration.Round(time.Millisecond))
	if report.Score > 0 {
		fmt.Printf("| CVSS Score | %.1f (%s) |\n", report.Score, report.Severity)
	}
	fmt.Printf("| Result | %s |\n", getStatusText(report.Status))
	fmt.Println()
	if report.Details != "" {
		fmt.Println("## Script Execution Output")
		fmt.Println()
		fmt.Println("```")
		fmt.Println(report.Details)
		fmt.Println("```")
		fmt.Println()
	}
	if report.AIAnalysis != "" {
		fmt.Println("## AI Analysis & Recommendations")
		fmt.Println()
		fmt.Println(report.AIAnalysis)
		fmt.Println()
	}
	if report.Script != "" {
		fmt.Println("## Generated Script")
		fmt.Println()
		fmt.Println("```bash")
		fmt.Println(report.Script)
		fmt.Println("```")
	}
}

// getStatusText 获取状态文本。
func getStatusText(status domain.VulnResult) string {
	switch status {
	case domain.VulnResultSafe:
		return "✅ 安全"
	case domain.VulnResultVulnerable:
		return "⚠️ 存在漏洞"
	case domain.VulnResultError:
		return "❌ 错误"
	default:
		return "❓ 未知"
	}
}

// truncate 截断字符串。
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// rulesListAction 列出所有安全规则。
func rulesListAction(c *cli.Context) error {
	// TODO: 获取安全扫描器并列出规则
	fmt.Printf("Security Rules:\n")
	fmt.Println("─────────────────────────────────────────")
	fmt.Printf("  [Implementation needed]\n")

	return nil
}

// rulesAddAction 添加安全规则。
func rulesAddAction(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("rule name is required")
	}

	pattern := c.Args().Get(1)
	if pattern == "" {
		return fmt.Errorf("rule pattern is required")
	}

	// TODO: 添加安全规则
	fmt.Printf("Rule added: %s\n", name)
	return nil
}

// rulesRemoveAction 移除安全规则。
func rulesRemoveAction(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("rule name is required")
	}

	// TODO: 移除安全规则
	fmt.Printf("Rule removed: %s\n", name)
	return nil
}
