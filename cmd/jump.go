package cmd

import (
	"fmt"
	"strconv"

	"assh/internal/i18n"

	"github.com/urfave/cli"
)

// registerJumpCommands 注册跳板相关命令（history）。
func (a *App) registerJumpCommands() {
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:  "jump",
		Usage: i18n.T("cmd.jump.description"),
		Subcommands: []cli.Command{
			{
				Name:      "history",
				Usage:     i18n.T("cmd.jump.list.description"),
				ArgsUsage: "[list]",
				Description: i18n.T("cmd.jump.description_long"),
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "clear", Usage: i18n.T("cmd.jump.flag.clear")},
					cli.StringFlag{Name: "delete", Usage: i18n.T("cmd.jump.flag.delete")},
					cli.IntFlag{Name: "n", Value: 10, Usage: i18n.T("cmd.jump.flag.limit")},
				},
				Action: a.jumpHistoryAction,
			},
		},
	})
}

// jumpHistoryAction 处理 jump history 子命令。
func (a *App) jumpHistoryAction(c *cli.Context) error {
	if a.jumpSvc == nil {
		return fmt.Errorf("%s", i18n.T("cmd.jump.error.service_unavailable"))
	}

	// --clear: 清空历史
	if c.Bool("clear") {
		if err := a.jumpSvc.Clear(); err != nil {
			return fmt.Errorf("%s", i18n.T("cmd.jump.error.clear_failed", map[string]interface{}{"Error": err}))
		}
		fmt.Println(i18n.T("cmd.jump.cleared"))
		return nil
	}

	// --delete <id>: 删除指定记录
	if deleteID := c.String("delete"); deleteID != "" {
		id, err := strconv.ParseInt(deleteID, 10, 64)
		if err != nil {
			return fmt.Errorf("%s", i18n.T("cmd.jump.error.invalid_id", map[string]interface{}{"ID": deleteID}))
		}
		if err := a.jumpSvc.Delete(id); err != nil {
			return fmt.Errorf("%s", i18n.T("cmd.jump.error.delete_failed", map[string]interface{}{"ID": id, "Error": err}))
		}
		fmt.Println(i18n.T("cmd.jump.deleted", map[string]interface{}{"ID": id}))
		return nil
	}

	// 默认：列出历史
	limit := c.Int("n")
	if limit <= 0 {
		limit = 10
	}

	records, err := a.jumpSvc.List(limit)
	if err != nil {
		return fmt.Errorf("%s", i18n.T("cmd.jump.error.list_failed", map[string]interface{}{"Error": err}))
	}

	if len(records) == 0 {
		fmt.Println(i18n.T("cmd.jump.list.empty"))
		return nil
	}

	// 表头
	fmt.Printf("%-4s  %-24s  %-32s  %-4s  %-4s  %-20s\n",
		i18n.T("cmd.jump.list.header.id"),
		i18n.T("cmd.jump.list.header.target"),
		i18n.T("cmd.jump.list.header.path"),
		i18n.T("cmd.jump.list.header.hops"),
		i18n.T("cmd.jump.list.header.use"),
		i18n.T("cmd.jump.list.header.last_used"))
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
	fmt.Printf("\n%s\n", i18n.T("cmd.jump.list.total", map[string]interface{}{"Count": len(records)}))

	return nil
}
