package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"assh/asshc/domain"

	"github.com/urfave/cli"
)

// registerSyncCommands 注册 sync 子命令。
//
// 同步命令用于管理云同步，包含以下子命令：
//   - account: 配置云账户
//   - push: 推送本地配置到云端
//   - pull: 从云端拉取配置
//   - history: 查看同步历史
func (a *App) registerSyncCommands() {
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:  "sync",
		Usage: "Cloud synchronization of server configurations",
		Subcommands: []cli.Command{
			{
				Name:  "account",
				Usage: "Configure cloud sync account",
				Subcommands: []cli.Command{
					{
						Name:   "set",
						Usage:  "Configure or update cloud account (interactive)",
						Action: a.syncAccountSetAction,
						Flags: []cli.Flag{
							cli.StringFlag{Name: "access-key", Usage: "Qiniu Access Key"},
							cli.StringFlag{Name: "secret-key", Usage: "Qiniu Secret Key"},
							cli.StringFlag{Name: "bucket", Usage: "Qiniu bucket name"},
							cli.StringFlag{Name: "zone", Usage: "Qiniu zone (huadong/huabei/huanan/beimei/xinjiapo)"},
						},
					},
					{
						Name:   "show",
						Usage:  "Show current cloud account info",
						Action: a.syncAccountShowAction,
					},
					{
						Name:   "delete",
						Usage:  "Delete cloud account configuration",
						Action: a.syncAccountDeleteAction,
					},
					{
						Name:   "test",
						Usage:  "Test cloud account connection",
						Action: a.syncAccountTestAction,
					},
				},
			},
			{
				Name:   "push",
				Usage:  "Push local server configs to cloud",
				Action: a.syncPushAction,
			},
			{
				Name:   "pull",
				Usage:  "Pull server configs from cloud",
				Action: a.syncPullAction,
			},
			{
				Name:   "history",
				Usage:  "Show sync history",
				Action: a.syncHistoryAction,
				Flags: []cli.Flag{
					cli.IntFlag{Name: "n", Value: 10, Usage: "number of history entries to show"},
				},
			},
		},
	})
}

// syncAccountSetAction 交互式配置或更新云账户信息。
//
// 支持两种模式：
//  1. 非交互模式：通过 --access-key/--secret-key/--bucket/--zone 参数直接设置
//  2. 交互模式：无参数时，逐项提示用户输入
func (a *App) syncAccountSetAction(c *cli.Context) error {
	if a.syncSvc == nil {
		return fmt.Errorf("sync service not available")
	}

	accessKey := c.String("access-key")
	secretKey := c.String("secret-key")
	bucket := c.String("bucket")
	zone := c.String("zone")

	// 如果未提供参数，使用交互模式
	if accessKey == "" || secretKey == "" || bucket == "" {
		fmt.Println("Configuring cloud sync account (Qiniu)")
		fmt.Println("Enter values (press Enter to keep current value if shown):")
		fmt.Println()

		// 读取当前配置（如果有）
		current, _ := a.syncSvc.GetAccount()

		reader := bufio.NewReader(os.Stdin)

		// AccessKey
		prompt := "Access Key"
		if current != nil && current.AccessKey != "" {
			prompt += " [" + maskString(current.AccessKey, 4) + "]"
		}
		prompt += ": "
		fmt.Print(prompt)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			accessKey = input
		} else if current != nil && current.AccessKey != "" {
			accessKey = current.AccessKey
		}

		// SecretKey
		prompt = "Secret Key"
		if current != nil && current.SecretKey != "" {
			prompt += " [already set]"
		}
		prompt += ": "
		fmt.Print(prompt)
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			secretKey = input
		} else if current != nil && current.SecretKey != "" {
			secretKey = current.SecretKey
		}

		// Bucket
		prompt = "Bucket"
		if current != nil && current.Bucket != "" {
			prompt += " [" + current.Bucket + "]"
		}
		prompt += ": "
		fmt.Print(prompt)
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			bucket = input
		} else if current != nil && current.Bucket != "" {
			bucket = current.Bucket
		}

		// Zone
		prompt = "Zone (huadong/huabei/huanan/beimei/xinjiapo)"
		if current != nil && current.Zone != "" {
			prompt += " [" + current.Zone + "]"
		} else {
			prompt += " [huadong]"
		}
		prompt += ": "
		fmt.Print(prompt)
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			zone = input
		} else if current != nil && current.Zone != "" {
			zone = current.Zone
		} else {
			zone = "huadong"
		}
	}

	if accessKey == "" || secretKey == "" || bucket == "" {
		return fmt.Errorf("access key, secret key, and bucket are required")
	}

	if zone == "" {
		zone = "huadong"
	}

	acct := &domain.CloudAccount{
		Name:      "default",
		Type:      "qiniu",
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    bucket,
		Zone:      zone,
		Enabled:   true,
	}

	if err := a.syncSvc.SetAccount(acct); err != nil {
		return fmt.Errorf("failed to save account: %w", err)
	}

	fmt.Println("Cloud account configured successfully.")
	return nil
}

// syncAccountShowAction 显示当前云账户信息（不显示 SecretKey）。
func (a *App) syncAccountShowAction(c *cli.Context) error {
	if a.syncSvc == nil {
		return fmt.Errorf("sync service not available")
	}

	acct, err := a.syncSvc.GetAccount()
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	fmt.Println("Cloud Sync Account:")
	fmt.Printf("  Type:      %s\n", acct.Type)
	fmt.Printf("  AccessKey: %s\n", acct.AccessKey)
	fmt.Printf("  SecretKey: %s\n", maskString(acct.SecretKey, 4))
	fmt.Printf("  Bucket:    %s\n", acct.Bucket)
	fmt.Printf("  Zone:      %s\n", acct.Zone)
	fmt.Printf("  Enabled:   %v\n", acct.Enabled)

	return nil
}

// syncAccountDeleteAction 删除云账户配置。
func (a *App) syncAccountDeleteAction(c *cli.Context) error {
	if a.syncSvc == nil {
		return fmt.Errorf("sync service not available")
	}

	fmt.Print("Are you sure you want to delete the cloud account? [y/N]: ")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
		fmt.Println("Canceled.")
		return nil
	}

	if err := a.syncSvc.DeleteAccount(); err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	fmt.Println("Cloud account deleted.")
	return nil
}

// syncAccountTestAction 测试云账户连接。
func (a *App) syncAccountTestAction(c *cli.Context) error {
	if a.syncSvc == nil {
		return fmt.Errorf("sync service not available")
	}

	fmt.Println("Testing cloud account connection...")

	if err := a.syncSvc.TestAccount(); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	fmt.Println("Connection successful!")
	return nil
}

// syncPushAction 推送本地服务器配置到云端。
func (a *App) syncPushAction(c *cli.Context) error {
	if a.syncSvc == nil {
		return fmt.Errorf("sync service not available")
	}

	fmt.Println("Pushing server configurations to cloud...")

	ctx := context.Background()
	result, err := a.syncSvc.Push(ctx)
	if err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	fmt.Printf("Push completed: %s\n", result.Message)
	return nil
}

// syncPullAction 从云端拉取服务器配置。
func (a *App) syncPullAction(c *cli.Context) error {
	if a.syncSvc == nil {
		return fmt.Errorf("sync service not available")
	}

	fmt.Println("Pulling server configurations from cloud...")

	ctx := context.Background()
	result, err := a.syncSvc.Pull(ctx)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	fmt.Printf("Pull completed: %s\n", result.Message)
	if result.Conflicts > 0 {
		fmt.Printf("  Conflicts: %d (cloud versions saved with _cloud_<ts> suffix)\n", result.Conflicts)
	}
	return nil
}

// syncHistoryAction 显示同步历史记录。
func (a *App) syncHistoryAction(c *cli.Context) error {
	if a.syncSvc == nil {
		return fmt.Errorf("sync service not available")
	}

	limit := c.Int("n")
	histories, err := a.syncSvc.GetSyncHistory(limit)
	if err != nil {
		return fmt.Errorf("failed to get sync history: %w", err)
	}

	if len(histories) == 0 {
		fmt.Println("No sync history.")
		return nil
	}

	fmt.Println("Sync History:")
	for _, h := range histories {
		direction := "↑ Push"
		if h.Direction == domain.SyncDirectionPull {
			direction = "↓ Pull"
		}
		status := "✓"
		if h.Status == domain.SyncStatusFailed {
			status = "✗"
		} else if h.Status == domain.SyncStatusPartial {
			status = "~"
		}

		fmt.Printf("  %s %s %s", status, direction, h.Timestamp.Format("2006-01-02 15:04:05"))
		if h.Message != "" {
			fmt.Printf("  %s", h.Message)
		}
		fmt.Println()
	}

	return nil
}

// maskString 将字符串中间部分替换为星号，只保留前 N 个字符。
// 用于安全显示密钥等敏感信息。
func maskString(s string, visible int) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) <= visible+4 {
		return s[:visible] + "****"
	}
	return s[:visible] + "****" + s[len(s)-4:]
}
