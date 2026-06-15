package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"assh/asshc/port"
	"assh/internal/i18n"

	"github.com/urfave/cli"
)

// registerHealthCommands 注册健康检查相关命令。
func (a *App) registerHealthCommands() {
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:  "health",
		Usage: "Server health check",
		Description: i18n.T("cmd.health.description"),
		Subcommands: []cli.Command{
			{
				Name:      "check",
				Usage:     "Check single or multiple servers",
				ArgsUsage: "<server>[,<server>...]",
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "detail, d", Usage: i18n.T("cmd.health.flag.details")},
				},
				Action: a.healthCheckAction,
			},
			{
				Name:  "list",
				Usage: "Check all servers or a group",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "group, g", Usage: i18n.T("cmd.health.flag.group")},
					cli.BoolFlag{Name: "detail, d", Usage: i18n.T("cmd.health.flag.details")},
					cli.IntFlag{Name: "concurrency, c", Value: 5, Usage: i18n.T("cmd.health.flag.concurrency")},
				},
				Action: a.healthListAction,
			},
		},
	})
}

// healthCheckAction 处理 `assh health check` 命令。
// 支持逗号分隔的多服务器检查。
func (a *App) healthCheckAction(c *cli.Context) error {
	if a.healthSvc == nil {
		return fmt.Errorf("%s", i18n.T("cmd.health.error.service_unavailable"))
	}

	if c.NArg() < 1 {
		return cli.ShowSubcommandHelp(c)
	}

	detail := c.Bool("detail")
	names := strings.Split(c.Args()[0], ",")

	// 清理名称
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}

	if len(names) == 1 {
		// 单一服务器检查
		return a.checkSingleServer(names[0], detail)
	}

	// 多服务器检查
	results, err := a.healthSvc.CheckServers(names, detail, 5)
	if err != nil {
		return err
	}

	a.printHealthResults(results, detail)
	return nil
}

// checkSingleServer 检查单台服务器并输出详细结果。
func (a *App) checkSingleServer(name string, detail bool) error {
	result, err := a.healthSvc.CheckServer(name, detail)
	if err != nil {
		return err
	}

	a.printSingleResult(result, detail)
	return nil
}

// healthListAction 处理 `assh health list` 命令。
func (a *App) healthListAction(c *cli.Context) error {
	if a.healthSvc == nil {
		return fmt.Errorf("%s", i18n.T("cmd.health.error.service_unavailable"))
	}

	detail := c.Bool("detail")
	concurrency := c.Int("concurrency")
	group := c.String("group")

	var results []*port.HealthResult
	var err error

	if group != "" {
		results, err = a.healthSvc.CheckGroup(group, detail, concurrency)
	} else {
		results, err = a.healthSvc.CheckAll(detail, concurrency)
	}

	if err != nil {
		return err
	}

	a.printHealthResults(results, detail)
	return nil
}

// printSingleResult 输出单台服务器的检查结果。
func (a *App) printSingleResult(r *port.HealthResult, detail bool) {
	statusIcon := statusIcon(r.Status)
	fmt.Println(i18n.T("cmd.health.result.server", map[string]interface{}{"Icon": statusIcon, "Server": r.Server, "Host": r.Host, "Port": r.Port}))
	fmt.Println(i18n.T("cmd.health.result.status", map[string]interface{}{"Status": r.Status}))
	fmt.Println(i18n.T("cmd.health.result.latency", map[string]interface{}{"Latency": formatDuration(r.Latency)}))
	fmt.Println(i18n.T("cmd.health.result.checked", map[string]interface{}{"Checked": r.CheckedAt.Format("2006-01-02 15:04:05")}))

	if r.Error != "" {
		fmt.Println(i18n.T("cmd.health.result.error", map[string]interface{}{"Error": r.Error}))
	}

	if detail && r.Details != nil {
		fmt.Println(i18n.T("cmd.health.result.system_info"))
		if r.Details.Uptime != "" {
			fmt.Println(i18n.T("cmd.health.result.uptime", map[string]interface{}{"Uptime": r.Details.Uptime}))
		}
		if r.Details.LoadAvg != "" {
			fmt.Println(i18n.T("cmd.health.result.load", map[string]interface{}{"Load": r.Details.LoadAvg}))
		}
		if r.Details.Memory != "" {
			fmt.Println(i18n.T("cmd.health.result.memory", map[string]interface{}{"Memory": r.Details.Memory}))
		}
		if r.Details.Disk != "" {
			fmt.Println(i18n.T("cmd.health.result.disk", map[string]interface{}{"Disk": r.Details.Disk}))
		}
	}
}

// printHealthResults 以表格形式输出多台服务器的检查结果。
func (a *App) printHealthResults(results []*port.HealthResult, detail bool) {
	if detail {
		// 详细模式：每台服务器单独输出
		for i, r := range results {
			if i > 0 {
				fmt.Println()
			}
			a.printSingleResult(r, true)
		}

		// 在详细模式底部也显示统计摘要
		printHealthSummary(results)
		return
	}

	// 表格模式
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "%s\n", i18n.T("cmd.health.list.header"))
	fmt.Fprintf(w, "------\t------\t-------\t-----\n")
	for _, r := range results {
		errMsg := r.Error
		if len(errMsg) > 50 {
			errMsg = errMsg[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			r.Server, statusIcon(r.Status), formatDuration(r.Latency), errMsg)
	}

	w.Flush()

	// 统计摘要
	printHealthSummary(results)
}

// printHealthSummary 输出健康检查的统计摘要。
func printHealthSummary(results []*port.HealthResult) {
	var healthy, unhealthy, timeout, errCount int
	for _, r := range results {
		switch r.Status {
		case port.HealthStatusHealthy:
			healthy++
		case port.HealthStatusUnhealthy:
			unhealthy++
		case port.HealthStatusTimeout:
			timeout++
		case port.HealthStatusError:
			errCount++
		}
	}

	fmt.Printf("\n%s", i18n.T("cmd.health.list.total", map[string]interface{}{"Count": len(results)}))
	fmt.Printf("%s", i18n.T("cmd.health.list.healthy", map[string]interface{}{"Count": healthy}))
	if unhealthy > 0 {
		fmt.Printf(" | %s", i18n.T("cmd.health.list.unhealthy", map[string]interface{}{"Count": unhealthy}))
	}
	if timeout > 0 {
		fmt.Printf(" | %s", i18n.T("cmd.health.list.timeout", map[string]interface{}{"Count": timeout}))
	}
	if errCount > 0 {
		fmt.Printf(" | %s", i18n.T("cmd.health.list.error", map[string]interface{}{"Count": errCount}))
	}
	fmt.Println()
}

// statusIcon 返回状态对应的图标。
func statusIcon(s port.HealthStatus) string {
	switch s {
	case port.HealthStatusHealthy:
		return "✓"
	case port.HealthStatusUnhealthy:
		return "⚠"
	case port.HealthStatusTimeout:
		return "⏱"
	case port.HealthStatusError:
		return "✗"
	default:
		return "?"
	}
}

// formatDuration 格式化时间延迟。
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.0fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
