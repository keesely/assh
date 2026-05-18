package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"assh/asshc/port"

	"github.com/urfave/cli"
)

// registerHealthCommands 注册健康检查相关命令。
func (a *App) registerHealthCommands() {
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:  "health",
		Usage: "Server health check",
		Description: `Check server health status via SSH connection.

Examples:
  assh health check myserver                  # Single server check
  assh health check myserver --detail         # With system details
  assh health check server1,server2           # Multiple servers
  assh health list                            # Check all servers
  assh health list --group prod               # Check group
  assh health list --concurrency 10           # With concurrency limit
  assh health list --detail                   # With system details
`,
		Subcommands: []cli.Command{
			{
				Name:      "check",
				Usage:     "Check single or multiple servers",
				ArgsUsage: "<server>[,<server>...]",
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "detail, d", Usage: "show system details (uptime/load/memory/disk)"},
				},
				Action: a.healthCheckAction,
			},
			{
				Name:  "list",
				Usage: "Check all servers or a group",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "group, g", Usage: "check specific group only"},
					cli.BoolFlag{Name: "detail, d", Usage: "show system details"},
					cli.IntFlag{Name: "concurrency, c", Value: 5, Usage: "max concurrent checks"},
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
		return fmt.Errorf("health service not available")
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
		return fmt.Errorf("health service not available")
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
	fmt.Printf("%s %s (%s:%d)\n", statusIcon, r.Server, r.Host, r.Port)
	fmt.Printf("  Status:   %s\n", r.Status)
	fmt.Printf("  Latency:  %s\n", formatDuration(r.Latency))
	fmt.Printf("  Checked:  %s\n", r.CheckedAt.Format("2006-01-02 15:04:05"))

	if r.Error != "" {
		fmt.Printf("  Error:    %s\n", r.Error)
	}

	if detail && r.Details != nil {
		fmt.Println("  --- System Info ---")
		if r.Details.Uptime != "" {
			fmt.Printf("  Uptime:   %s\n", r.Details.Uptime)
		}
		if r.Details.LoadAvg != "" {
			fmt.Printf("  Load:     %s\n", r.Details.LoadAvg)
		}
		if r.Details.Memory != "" {
			fmt.Printf("  Memory:   %s\n", r.Details.Memory)
		}
		if r.Details.Disk != "" {
			fmt.Printf("  Disk:     %s\n", r.Details.Disk)
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

	fmt.Fprintf(w, "SERVER\tSTATUS\tLATENCY\tERROR\n")
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

	fmt.Printf("\nTotal: %d | ", len(results))
	fmt.Printf("✓ %d healthy", healthy)
	if unhealthy > 0 {
		fmt.Printf(" | ⚠ %d unhealthy", unhealthy)
	}
	if timeout > 0 {
		fmt.Printf(" | ⏱ %d timeout", timeout)
	}
	if errCount > 0 {
		fmt.Printf(" | ✗ %d error", errCount)
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
