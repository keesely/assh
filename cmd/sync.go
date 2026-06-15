package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"assh/asshc/domain"
	"assh/internal/i18n"

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
		Usage: i18n.T("cmd.sync.description"),
		Subcommands: []cli.Command{
			{
				Name:  "account",
				Usage: i18n.T("cmd.sync.account.description"),
				Subcommands: []cli.Command{
					{
						Name:   "set",
						Usage:  i18n.T("cmd.sync.account.set.description"),
						Action: a.syncAccountSetAction,
						Flags: []cli.Flag{
							cli.StringFlag{Name: "access-key", Usage: i18n.T("cmd.sync.flag.access_key")},
							cli.StringFlag{Name: "secret-key", Usage: i18n.T("cmd.sync.flag.secret_key")},
							cli.StringFlag{Name: "bucket", Usage: i18n.T("cmd.sync.flag.bucket")},
							cli.StringFlag{Name: "zone", Usage: i18n.T("cmd.sync.flag.zone")},
						},
					},
					{
						Name:   "show",
						Usage:  i18n.T("cmd.sync.account.show.description"),
						Action: a.syncAccountShowAction,
					},
					{
						Name:   "delete",
						Usage:  i18n.T("cmd.sync.account.delete.description"),
						Action: a.syncAccountDeleteAction,
					},
					{
						Name:   "test",
						Usage:  i18n.T("cmd.sync.account.test.description"),
						Action: a.syncAccountTestAction,
					},
				},
			},
			{
				Name:   "push",
				Usage:  i18n.T("cmd.sync.push.description"),
				Action: a.syncPushAction,
			},
			{
				Name:   "pull",
				Usage:  i18n.T("cmd.sync.pull.description"),
				Action: a.syncPullAction,
			},
			{
				Name:   "history",
				Usage:  i18n.T("cmd.sync.history.description"),
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
		fmt.Println(i18n.T("cmd.sync.account.configuring"))
		fmt.Println(i18n.T("cmd.sync.account.enter_values"))
		fmt.Println()

		// 读取当前配置（如果有）
		current, _ := a.syncSvc.GetAccount()

		reader := bufio.NewReader(os.Stdin)

		// AccessKey
		prompt := i18n.T("cmd.sync.account.prompt.access_key")
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
		prompt = i18n.T("cmd.sync.account.prompt.secret_key")
		if current != nil && current.SecretKey != "" {
			prompt += " " + i18n.T("cmd.sync.account.already_set")
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
		prompt = i18n.T("cmd.sync.account.prompt.bucket")
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
		prompt = i18n.T("cmd.sync.account.prompt.zone")
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
		return fmt.Errorf("%s", i18n.T("cmd.sync.account.error.required"))
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

	fmt.Println(i18n.T("cmd.sync.account.success"))
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

	fmt.Println(i18n.T("cmd.sync.account.info"))
	fmt.Printf("%s      %s\n", i18n.T("cmd.sync.account.type"), acct.Type)
	fmt.Printf("%s %s\n", i18n.T("cmd.sync.account.access_key"), acct.AccessKey)
	fmt.Printf("%s %s\n", i18n.T("cmd.sync.account.secret_key"), maskString(acct.SecretKey, 4))
	fmt.Printf("%s    %s\n", i18n.T("cmd.sync.account.bucket"), acct.Bucket)
	fmt.Printf("%s      %s\n", i18n.T("cmd.sync.account.zone"), acct.Zone)
	fmt.Printf("%s   %v\n", i18n.T("cmd.sync.account.enabled"), acct.Enabled)

	return nil
}

// syncAccountDeleteAction 删除云账户配置。
func (a *App) syncAccountDeleteAction(c *cli.Context) error {
	if a.syncSvc == nil {
		return fmt.Errorf("sync service not available")
	}

	fmt.Print(i18n.T("cmd.sync.account.confirm_delete"))
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
		fmt.Println(i18n.T("common.cancelled"))
		return nil
	}

	if err := a.syncSvc.DeleteAccount(); err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	fmt.Println(i18n.T("cmd.sync.account.deleted"))
	return nil
}

// syncAccountTestAction 测试云账户连接。
func (a *App) syncAccountTestAction(c *cli.Context) error {
	if a.syncSvc == nil {
		return fmt.Errorf("sync service not available")
	}

	fmt.Println(i18n.T("cmd.sync.account.testing"))

	if err := a.syncSvc.TestAccount(); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	fmt.Println(i18n.T("cmd.sync.account.test_success"))
	return nil
}

// syncPushAction 推送本地服务器配置到云端。
func (a *App) syncPushAction(c *cli.Context) error {
	if a.syncSvc == nil {
		return fmt.Errorf("sync service not available")
	}

	fmt.Println(i18n.T("cmd.sync.push.pushing"))

	ctx := context.Background()
	result, err := a.syncSvc.Push(ctx)
	if err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	fmt.Printf("%s %s\n", i18n.T("cmd.sync.push.completed"), result.Message)
	return nil
}

// syncPullAction 从云端拉取服务器配置。
func (a *App) syncPullAction(c *cli.Context) error {
	if a.syncSvc == nil {
		return fmt.Errorf("sync service not available")
	}

	fmt.Println(i18n.T("cmd.sync.pull.pulling"))

	ctx := context.Background()
	result, err := a.syncSvc.Pull(ctx)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	fmt.Printf("%s %s\n", i18n.T("cmd.sync.pull.completed"), result.Message)
	if result.Conflicts > 0 {
		fmt.Println(i18n.T("cmd.sync.pull.conflicts", map[string]interface{}{"Count": result.Conflicts}))
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
		fmt.Println(i18n.T("cmd.sync.history.empty"))
		return nil
	}

	fmt.Println(i18n.T("cmd.sync.history.title"))
	for _, h := range histories {
		direction := "↑ " + i18n.T("cmd.sync.direction.push")
		if h.Direction == domain.SyncDirectionPull {
			direction = "↓ " + i18n.T("cmd.sync.direction.pull")
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
		return i18n.T("common.not_set")
	}
	if len(s) <= visible+4 {
		return s[:visible] + "****"
	}
	return s[:visible] + "****" + s[len(s)-4:]
}
