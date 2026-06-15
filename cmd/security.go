package cmd

import (
	"fmt"
	"strings"
	"time"

	"assh/asshc/domain"
	"assh/internal/i18n"
	"github.com/urfave/cli"
)

// SecurityCommand 定义安全检查命令。
func SecurityCommand() cli.Command {
	return cli.Command{
		Name:  "security",
		Usage: i18n.T("cmd.security.description"),
		Subcommands: []cli.Command{
			{
				Name:  "vuln-check",
				Usage: i18n.T("cmd.security.check.description"),
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "server, s",
						Usage: i18n.T("cmd.security.flag.server"),
					},
					cli.StringFlag{
						Name:  "group, g",
						Usage: i18n.T("cmd.security.flag.group"),
					},
					cli.StringFlag{
						Name:  "output, o",
						Usage: i18n.T("cmd.security.flag.format"),
						Value: "text",
					},
					cli.BoolFlag{
						Name:  "no-execute",
						Usage: i18n.T("cmd.security.flag.dry_run"),
					},
					cli.BoolFlag{
						Name:  "no-confirm",
						Usage: i18n.T("cmd.security.flag.skip_confirm"),
					},
					cli.DurationFlag{
						Name:  "timeout",
						Usage: i18n.T("cmd.security.flag.timeout"),
						Value: 300 * time.Second,
					},
				},
				Action: vulnCheckAction,
			},
			{
				Name:  "rules",
				Usage: i18n.T("cmd.security.rules.description"),
				Subcommands: []cli.Command{
					{
						Name:   "list",
						Usage:  i18n.T("cmd.security.rules.list.description"),
						Action: rulesListAction,
					},
					{
						Name:   "add",
						Usage:  i18n.T("cmd.security.rules.add.description"),
						Action: rulesAddAction,
					},
					{
						Name:   "remove",
						Usage:  i18n.T("cmd.security.rules.remove.description"),
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
		return fmt.Errorf("%s", i18n.T("cmd.security.error.vuln_required"))
	}

	if server == "" && group == "" {
		return fmt.Errorf("%s", i18n.T("cmd.security.error.target_required"))
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
			fmt.Println(i18n.T("cmd.security.checking_server", map[string]interface{}{"Server": s, "Vuln": vulnInfo}))
		}
	} else if group != "" {
		// 检查分组服务器
		fmt.Println(i18n.T("cmd.security.checking_group", map[string]interface{}{"Group": group, "Vuln": vulnInfo}))
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
	fmt.Println(i18n.T("cmd.security.vuln.title"))
	fmt.Println(i18n.T("cmd.security.vuln.vulnerability"), report.VulnID)
	fmt.Println(i18n.T("cmd.security.vuln.target"), report.Target)
	fmt.Println(i18n.T("cmd.security.vuln.status"), getStatusText(report.Status))
	fmt.Println(i18n.T("cmd.security.vuln.details"), truncate(report.Details, 28))
	fmt.Println(i18n.T("cmd.security.vuln.suggestion"), truncate(report.Suggestion, 28))
}

// outputJSON 输出 JSON 格式。
func outputJSON(report *domain.VulnReport) {
	fmt.Println(i18n.T("cmd.security.vuln.json_not_impl"))
}

// outputMarkdown 输出 Markdown 格式。
func outputMarkdown(report *domain.VulnReport) {
	fmt.Printf("# %s\n\n", i18n.T("cmd.security.vuln.report_title"))
	fmt.Printf("- %s %s\n", i18n.T("cmd.security.vuln.vulnerability"), report.VulnID)
	fmt.Printf("- %s %s\n", i18n.T("cmd.security.vuln.target"), report.Target)
	fmt.Printf("- %s %s\n", i18n.T("cmd.security.vuln.time"), report.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("- %s %s\n", i18n.T("cmd.security.vuln.status"), getStatusText(report.Status))
	fmt.Printf("- %s %s\n", i18n.T("cmd.security.vuln.details"), report.Details)
	fmt.Printf("- %s %s\n", i18n.T("cmd.security.vuln.suggestion"), report.Suggestion)
}

// getStatusText 获取状态文本。
func getStatusText(status domain.VulnResult) string {
	switch status {
	case domain.VulnResultSafe:
		return i18n.T("cmd.security.vuln.status_safe")
	case domain.VulnResultVulnerable:
		return i18n.T("cmd.security.vuln.status_vulnerable")
	case domain.VulnResultError:
		return i18n.T("cmd.security.vuln.status_error")
	default:
		return i18n.T("cmd.security.vuln.status_unknown")
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
	fmt.Println(i18n.T("cmd.security.rules.header"))
	fmt.Println("─────────────────────────────────────────")
	fmt.Println(i18n.T("cmd.security.rules.not_implemented"))

	return nil
}

// rulesAddAction 添加安全规则。
func rulesAddAction(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("%s", i18n.T("cmd.security.error.rule_name_required"))
	}

	pattern := c.Args().Get(1)
	if pattern == "" {
		return fmt.Errorf("%s", i18n.T("cmd.security.error.rule_pattern_required"))
	}

	fmt.Println(i18n.T("cmd.security.rules.added", map[string]interface{}{"Name": name}))
	return nil
}

// rulesRemoveAction 移除安全规则。
func rulesRemoveAction(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("%s", i18n.T("cmd.security.error.rule_name_required"))
	}

	fmt.Println(i18n.T("cmd.security.rules.removed", map[string]interface{}{"Name": name}))
	return nil
}
