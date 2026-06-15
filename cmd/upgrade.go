package cmd

import (
	"context"
	"fmt"
	"strings"

	"assh/asshc/domain"
	"assh/asshc/service"
	"assh/internal/i18n"

	"github.com/urfave/cli"
)

// registerUpgradeCommands 注册 upgrade 相关命令。
func (a *App) registerUpgradeCommands() {
	upgradeCmd := cli.Command{
		Name:    "upgrade",
		Aliases: []string{"update"},
		Usage:   "Check for updates and upgrade ASSH",
		Description: i18n.T("cmd.upgrade.description"),
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "check,c",
				Usage: i18n.T("cmd.upgrade.flag.check_only"),
			},
			cli.StringFlag{
				Name:  "version,v",
				Usage: i18n.T("cmd.upgrade.flag.version"),
			},
			cli.BoolFlag{
				Name:  "force,f",
				Usage: i18n.T("cmd.upgrade.flag.force"),
			},
			cli.BoolFlag{
				Name:  "rollback,r",
				Usage: i18n.T("cmd.upgrade.flag.rollback"),
			},
			cli.BoolFlag{
				Name:  "yes,y",
				Usage: i18n.T("cmd.upgrade.flag.yes"),
			},
		},
		Action: a.upgradeAction,
	}

	a.cli.Commands = append(a.cli.Commands, upgradeCmd)
}

// upgradeAction 处理 upgrade 命令。
func (a *App) upgradeAction(c *cli.Context) error {
	// 检查是否有升级服务
	if a.upgradeSvc == nil {
		return fmt.Errorf("%s", i18n.T("cmd.upgrade.error.service_unavailable"))
	}

	// 处理回滚
	if c.Bool("rollback") {
		return a.handleRollback()
	}

	// 处理仅检查
	if c.Bool("check") {
		return a.handleCheck()
	}

	// 处理升级
	return a.handleUpgrade(c)
}

// handleCheck 处理检查更新。
func (a *App) handleCheck() error {
	fmt.Println(i18n.T("cmd.upgrade.current_version", map[string]interface{}{"Version": a.upgradeSvc.GetCurrentVersion()}))
	fmt.Println(i18n.T("cmd.upgrade.checking"))

	info, err := a.upgradeSvc.CheckUpdate()
	if err != nil {
		if err == domain.ErrAlreadyLatest {
			fmt.Println(i18n.T("cmd.upgrade.up_to_date"))
			return nil
		}
		return fmt.Errorf("check failed: %w", err)
	}

	fmt.Println(i18n.T("cmd.upgrade.new_version", map[string]interface{}{"Version": info.Version}))
	fmt.Println(i18n.T("cmd.upgrade.release_date", map[string]interface{}{"Date": info.ReleaseDate.Format("2006-01-02 15:04:05")}))
	fmt.Println(i18n.T("cmd.upgrade.download_size", map[string]interface{}{"Size": formatSize(info.FileSize)}))

	if info.ReleaseNotes != "" {
		fmt.Printf("\n%s\n%s\n", i18n.T("cmd.upgrade.release_notes"), info.ReleaseNotes)
	}

	return nil
}

// handleRollback 处理回滚。
func (a *App) handleRollback() error {
	fmt.Println(i18n.T("cmd.upgrade.rolling_back"))

	if err := a.upgradeSvc.Rollback(); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	fmt.Println(i18n.T("cmd.upgrade.rollback_success"))
	return nil
}

// handleUpgrade 处理升级。
func (a *App) handleUpgrade(c *cli.Context) error {
	// 构建升级选项
	options := service.UpgradeOptions{
		Version: c.String("version"),
		Force:   c.Bool("force"),
		Yes:     c.Bool("yes"),
	}

	// 检查更新
	fmt.Println(i18n.T("cmd.upgrade.current_version", map[string]interface{}{"Version": a.upgradeSvc.GetCurrentVersion()}))

	if !options.Force || options.Version == "" {
		fmt.Println(i18n.T("cmd.upgrade.checking"))
		info, err := a.upgradeSvc.CheckUpdate()
		if err != nil {
			if err == domain.ErrAlreadyLatest {
				fmt.Println(i18n.T("cmd.upgrade.up_to_date"))
				return nil
			}
			return fmt.Errorf("check failed: %w", err)
		}

		fmt.Println(i18n.T("cmd.upgrade.new_version", map[string]interface{}{"Version": info.Version}))
		fmt.Println(i18n.T("cmd.upgrade.release_date", map[string]interface{}{"Date": info.ReleaseDate.Format("2006-01-02 15:04:05")}))
		fmt.Println(i18n.T("cmd.upgrade.download_size", map[string]interface{}{"Size": formatSize(info.FileSize)}))

		if !options.Yes {
			fmt.Print(i18n.T("cmd.upgrade.confirm"))
			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println(i18n.T("cmd.upgrade.cancelled"))
				return nil
			}
		}

		options.UpdateInfo = info
	}

	fmt.Println("\n" + i18n.T("cmd.upgrade.downloading"))
	progress := func(downloaded, total int64, percent float64) {
		fmt.Printf("\r%s", i18n.T("cmd.upgrade.downloading_progress", map[string]interface{}{"Percent": fmt.Sprintf("%.1f%%", percent*100), "Current": formatSize(downloaded), "Total": formatSize(total)}))
	}

	info, err := a.upgradeSvc.Upgrade(context.Background(), options, progress)
	if err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}

	fmt.Printf("\n\n%s\n", i18n.T("cmd.upgrade.success", map[string]interface{}{"Version": info.Version}))
	fmt.Println(i18n.T("cmd.upgrade.restart_hint"))

	return nil
}

// formatSize 格式化文件大小。
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
