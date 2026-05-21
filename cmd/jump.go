package cmd

import (
	"fmt"
	"strconv"

	"github.com/urfave/cli"
)

// registerJumpCommands 注册跳板相关命令（history）。
func (a *App) registerJumpCommands() {
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:  "jump",
		Usage: "Manage jump host history",
		Subcommands: []cli.Command{
			{
				Name:      "history",
				Usage:     "List jump host history",
				ArgsUsage: "[list]",
				Description: `Show recent jump host connection history.

Examples:
  assh jump history          # list recent jump history (default 10)
  assh jump history --clear  # clear all jump history
  assh jump history -n 20    # show last 20 records
`,
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "clear", Usage: "clear all jump history"},
					cli.StringFlag{Name: "delete", Usage: "delete a specific history entry by ID"},
					cli.IntFlag{Name: "n", Value: 10, Usage: "number of records to show"},
				},
				Action: a.jumpHistoryAction,
			},
		},
	})
}

// jumpHistoryAction 处理 jump history 子命令。
func (a *App) jumpHistoryAction(c *cli.Context) error {
	if a.jumpSvc == nil {
		return fmt.Errorf("jump service not available")
	}

	// --clear: 清空历史
	if c.Bool("clear") {
		if err := a.jumpSvc.Clear(); err != nil {
			return fmt.Errorf("failed to clear jump history: %w", err)
		}
		fmt.Println("jump history cleared")
		return nil
	}

	// --delete <id>: 删除指定记录
	if deleteID := c.String("delete"); deleteID != "" {
		id, err := strconv.ParseInt(deleteID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid ID: %s", deleteID)
		}
		if err := a.jumpSvc.Delete(id); err != nil {
			return fmt.Errorf("failed to delete jump history #%d: %w", id, err)
		}
		fmt.Printf("jump history #%d deleted\n", id)
		return nil
	}

	// 默认：列出历史
	limit := c.Int("n")
	if limit <= 0 {
		limit = 10
	}

	records, err := a.jumpSvc.List(limit)
	if err != nil {
		return fmt.Errorf("failed to list jump history: %w", err)
	}

	if len(records) == 0 {
		fmt.Println("No jump history records found")
		return nil
	}

	// 表头
	fmt.Printf("%-4s  %-24s  %-32s  %-4s  %-4s  %-20s\n",
		"ID", "TARGET", "PATH", "HOPS", "USE", "LAST USED")
	fmt.Println("────  ────────────────────────  ────────────────────────────────  ────  ────  ────────────────────")

	for _, r := range records {
		target := r.TargetExpr
		if len(target) > 24 {
			target = target[:21] + "..."
		}
		path := r.PathText
		if len(path) > 32 {
			path = path[:29] + "..."
		}
		lastUsed := r.LastUsed
		if len(lastUsed) > 19 {
			lastUsed = lastUsed[:19]
		}
		fmt.Printf("%-4d  %-24s  %-32s  %-4d  %-4d  %-20s\n",
			r.ID, target, path, r.ChainCount, r.UseCount, lastUsed)
	}
	fmt.Printf("\nTotal: %d record(s)\n", len(records))

	return nil
}
