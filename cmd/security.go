package cmd

import (
	"fmt"
	"strings"
	"time"

	"assh/asshc/domain"
	"github.com/urfave/cli"
)

// SecurityCommand 定义安全检查命令。
func SecurityCommand() cli.Command {
	return cli.Command{
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
				},
				Action: vulnCheckAction,
			},
			{
				Name:  "rules",
				Usage: "Manage security rules",
				Subcommands: []cli.Command{
					{
						Name:   "list",
						Usage:  "List all security rules",
						Action: rulesListAction,
					},
					{
						Name:   "add",
						Usage:  "Add a security rule",
						Action: rulesAddAction,
					},
					{
						Name:   "remove",
						Usage:  "Remove a security rule",
						Action: rulesRemoveAction,
					},
				},
			},
		},
	}
}

// vulnCheckAction 执行漏洞检查。
func vulnCheckAction(c *cli.Context) error {
	// 获取参数
	server := c.String("server")
	group := c.String("group")
	vulnInfo := c.Args().First()

	// 验证参数
	if vulnInfo == "" {
		return fmt.Errorf("vulnerability information is required")
	}

	if server == "" && group == "" {
		return fmt.Errorf("server or group is required")
	}

	// TODO: 获取应用组件并执行检查
	// 这里需要实现实际的检查逻辑

	if server != "" {
		// 检查单台或多台服务器
		servers := strings.Split(server, ",")
		for _, s := range servers {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}

			// TODO: 获取服务器配置并执行检查
			fmt.Printf("Checking server: %s for vulnerability: %s\n", s, vulnInfo)
		}
	} else if group != "" {
		// 检查分组服务器
		fmt.Printf("Checking group: %s for vulnerability: %s\n", group, vulnInfo)
	}

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
	fmt.Println("┌─ AI 安全检查 ─────────────────────┐")
	fmt.Printf("│ 漏洞: %-28s │\n", report.VulnID)
	fmt.Printf("│ 目标: %-28s │\n", report.Target)
	fmt.Printf("│ 状态: %-28s │\n", getStatusText(report.Status))
	fmt.Printf("│ 详情: %-28s │\n", truncate(report.Details, 28))
	fmt.Printf("│ 建议: %-28s │\n", truncate(report.Suggestion, 28))
	fmt.Println("└────────────────────────────────────┘")
}

// outputJSON 输出 JSON 格式。
func outputJSON(report *domain.VulnReport) {
	// TODO: 实现 JSON 输出
	fmt.Printf("JSON output not implemented yet\n")
}

// outputMarkdown 输出 Markdown 格式。
func outputMarkdown(report *domain.VulnReport) {
	fmt.Printf("# AI 安全检查报告\n\n")
	fmt.Printf("- **漏洞**: %s\n", report.VulnID)
	fmt.Printf("- **目标**: %s\n", report.Target)
	fmt.Printf("- **时间**: %s\n", report.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("- **状态**: %s\n", getStatusText(report.Status))
	fmt.Printf("- **详情**: %s\n", report.Details)
	fmt.Printf("- **建议**: %s\n", report.Suggestion)
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
