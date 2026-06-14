package cmd

import (
	"context"
	"fmt"
	"strings"

	"assh/asshc/domain"
	"assh/asshc/service"

	"github.com/urfave/cli"
)

// registerUpgradeCommands 注册 upgrade 相关命令。
func (a *App) registerUpgradeCommands() {
	upgradeCmd := cli.Command{
		Name:    "upgrade",
		Aliases: []string{"update"},
		Usage:   "Check for updates and upgrade ASSH",
		Description: `Check for new versions and upgrade ASSH to the latest version.

Examples:
  assh upgrade                    # Check and upgrade to latest
  assh upgrade --check            # Only check, don't upgrade
  assh upgrade --version v2.0.2   # Upgrade to specific version
  assh upgrade --force            # Force upgrade (skip version check)
  assh upgrade --rollback         # Rollback to previous version
  assh upgrade --source URL       # Custom update source
  assh upgrade -y                 # Auto-confirm (no prompt)`,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "check,c",
				Usage: "Only check for updates, don't upgrade",
			},
			cli.StringFlag{
				Name:  "version,v",
				Usage: "Upgrade to specific version",
			},
			cli.BoolFlag{
				Name:  "force,f",
				Usage: "Force upgrade (skip version check)",
			},
			cli.BoolFlag{
				Name:  "rollback,r",
				Usage: "Rollback to previous version",
			},
			cli.BoolFlag{
				Name:  "yes,y",
				Usage: "Auto-confirm (no prompt)",
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
		return fmt.Errorf("upgrade service not available")
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
	fmt.Printf("Current version: %s\n", a.upgradeSvc.GetCurrentVersion())
	fmt.Println("Checking for updates...")

	info, err := a.upgradeSvc.CheckUpdate()
	if err != nil {
		if err == domain.ErrAlreadyLatest {
			fmt.Println("✓ Already up to date!")
			return nil
		}
		return fmt.Errorf("check failed: %w", err)
	}

	fmt.Printf("\nNew version available: %s\n", info.Version)
	fmt.Printf("Release date: %s\n", info.ReleaseDate.Format("2006-01-02 15:04:05"))
	fmt.Printf("Download size: %s\n", formatSize(info.FileSize))

	if info.ReleaseNotes != "" {
		fmt.Printf("\nRelease notes:\n%s\n", info.ReleaseNotes)
	}

	return nil
}

// handleRollback 处理回滚。
func (a *App) handleRollback() error {
	fmt.Println("Rolling back to previous version...")

	if err := a.upgradeSvc.Rollback(); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	fmt.Println("✓ Rollback successful!")
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
	fmt.Printf("Current version: %s\n", a.upgradeSvc.GetCurrentVersion())

	if !options.Force || options.Version == "" {
		fmt.Println("Checking for updates...")
		info, err := a.upgradeSvc.CheckUpdate()
		if err != nil {
			if err == domain.ErrAlreadyLatest {
				fmt.Println("✓ Already up to date!")
				return nil
			}
			return fmt.Errorf("check failed: %w", err)
		}

		fmt.Printf("\nNew version available: %s\n", info.Version)
		fmt.Printf("Release date: %s\n", info.ReleaseDate.Format("2006-01-02 15:04:05"))
		fmt.Printf("Download size: %s\n", formatSize(info.FileSize))

		// 确认升级
		if !options.Yes {
			fmt.Print("\nDo you want to upgrade? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Upgrade cancelled.")
				return nil
			}
		}

		// 传递预先获取的 UpdateInfo，避免重复 API 调用
		options.UpdateInfo = info
	}

	// 执行升级
	fmt.Println("\nDownloading update...")
	progress := func(downloaded, total int64, percent float64) {
		fmt.Printf("\rDownloading: %.1f%% (%s/%s)", percent*100, formatSize(downloaded), formatSize(total))
	}

	info, err := a.upgradeSvc.Upgrade(context.Background(), options, progress)
	if err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}

	fmt.Printf("\n\n✓ Successfully upgraded to %s!\n", info.Version)
	fmt.Println("Please restart ASSH to use the new version.")

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
