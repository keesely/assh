// Package cmd 实现 CLI 命令层，基于 urfave/cli 框架。
//
// 本层负责命令注册、参数解析、全局标志处理和用户交互。
// 命令处理逻辑委托给 service 层，遵循"瘦 handler + 胖 service"模式。
package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"assh/asshc/domain"
	sshinfra "assh/asshc/infra/ssh"
	transferport "assh/asshc/port"
	"assh/asshc/service"
	"assh/config"
	"assh/internal/i18n"
	"assh/log"

	"github.com/pkg/sftp"
	"github.com/urfave/cli"
)

// recordDirectConnectAsync 异步记录直连操作到 known_servers 表。
// 传入预计算的 authFingerprint 避免明文密码在 goroutine 中传播。
// 封装为包级函数，使 fsPushAction / fsPullAction 可直接调用。
func recordDirectConnectAsync(kr transferport.KnownServerRecorder, host string, port int, user, authFingerprint string) {
	if kr == nil {
		return
	}
	id := domain.ComputeKnownServerID(user, host, port, authFingerprint)
	ks := &domain.KnownServer{
		ID:              id,
		Host:            host,
		Port:            port,
		User:            user,
		AuthFingerprint: authFingerprint,
	}
	recorder := kr
	go func() {
		if err := recorder.RecordDirectConnect(ks); err != nil {
			log.Debugf("failed to record direct connect: %v", err)
		}
	}()
}

// App 封装 CLI 应用的核心结构，持有 service 依赖并通过 urfave/cli 框架注册命令。
type App struct {
	cli            *cli.App                      // urfave/cli 应用实例
	version        string                        // 版本号（编译时注入）
	build          string                        // 构建信息（编译时注入）
	connectSvc     *service.ConnectService       // SSH 连接服务
	serverSvc      *service.ServerService        // 服务器配置管理服务
	knownRecorder  transferport.KnownServerRecorder // known-servers 隐性表记录器
	keySvc         *service.KeyService           // 密钥管理服务
	keymgr         transferport.KeyManager       // 密钥管理器（直连 keygen 用）
	proxySvc       *service.ProxyService         // 代理 & 隧道服务
	syncSvc        *service.SyncService          // 云同步服务
	healthSvc      *service.HealthService        // 健康检查服务
	jumpSvc        *service.JumpService          // 跳板历史服务
	upgradeSvc     *service.UpgradeService       // 自动更新服务 (Phase 11)
}

// NewApp 创建 CLI 应用，注入所有 service 依赖。
// 注册全局标志（-v/-q/-F/-V）和子命令（server/login/run/bc/keygen）。
func NewApp(version, build string, connectSvc *service.ConnectService, serverSvc *service.ServerService, knownRecorder transferport.KnownServerRecorder, keySvc *service.KeyService, keymgr transferport.KeyManager, proxySvc *service.ProxyService, syncSvc *service.SyncService, healthSvc *service.HealthService, jumpSvc *service.JumpService, upgradeSvc *service.UpgradeService) *App {
	app := cli.NewApp()
	app.Name = i18n.T("cli.name")
	app.Usage = i18n.T("cli.description")
	app.EnableBashCompletion = true
	app.HideVersion = true

	a := &App{
		cli:           app,
		version:       version,
		build:         build,
		connectSvc:    connectSvc,
		serverSvc:     serverSvc,
		knownRecorder: knownRecorder,
		keySvc:        keySvc,
		keymgr:        keymgr,
		proxySvc:      proxySvc,
		syncSvc:       syncSvc,
		healthSvc:     healthSvc,
		jumpSvc:       jumpSvc,
		upgradeSvc:    upgradeSvc,
	}
	app.Before = a.beforeAction
	a.setupGlobalFlags()
	app.Action = a.defaultAction
	a.registerCommands()

	// 为需要服务器名补全的命令注册 BashComplete
	a.registerCompletionHints()

	return a
}

// NewFSApp creates CLI application for file transfer (assh-fs binary).
func NewFSApp(version, build string, transferSvc *service.TransferService, serverSvc *service.ServerService, knownRecorder transferport.KnownServerRecorder, connectSvc *service.ConnectService, jumpSvc *service.JumpService) *App {
	app := cli.NewApp()
	app.Name = i18n.T("cli.fs_name")
	app.Usage = i18n.T("cli.fs_description")
	app.EnableBashCompletion = true
	app.HideVersion = true

	a := &App{
		cli:           app,
		version:       version,
		build:         build,
		connectSvc:    connectSvc,
		serverSvc:     serverSvc,
		knownRecorder: knownRecorder,
		jumpSvc:       jumpSvc,
	}
	app.Before = a.beforeAction
	a.setupGlobalFlags()
	a.registerFSCommands(transferSvc, serverSvc)

	return a
}

// registerFSCommands registers file transfer commands.
func (a *App) registerFSCommands(transferSvc *service.TransferService, serverSvc *service.ServerService) {
	kr := a.knownRecorder
	connectSvc := a.connectSvc
	a.cli.Commands = []cli.Command{
		{
			Name:   "version",
			Usage:  i18n.T("cli.cmd.version"),
			Action: a.versionAction,
		},
		{
			Name:   "push",
			Usage:  i18n.T("cli.cmd.push"),
			Action: fsPushAction(transferSvc, serverSvc, kr, connectSvc),
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "r, recursive", Usage: i18n.T("cli.flag.recursive")},
				cli.BoolFlag{Name: "e, resume", Usage: i18n.T("cli.flag.resume")},
				cli.BoolFlag{Name: "f, force", Usage: i18n.T("cli.flag.force")},
				cli.BoolFlag{Name: "skip", Usage: i18n.T("cli.flag.skip_existing")},
				cli.BoolFlag{Name: "S, checksum", Usage: i18n.T("cli.flag.verify")},
				cli.IntFlag{Name: "c, concurrency", Value: 3, Usage: i18n.T("cli.flag.concurrency")},
				// Direct connection flags (F12)
				cli.StringFlag{Name: "H, host", Usage: i18n.T("cli.flag.host")},
				cli.StringFlag{Name: "u, user", Usage: i18n.T("cli.flag.user")},
				cli.IntFlag{Name: "p, port", Value: 22, Usage: i18n.T("cli.flag.port")},
				cli.StringFlag{Name: "P, password", Usage: i18n.T("cli.flag.password")},
				cli.StringFlag{Name: "i, identity-file", Usage: i18n.T("cli.flag.identity_file")},
				cli.StringFlag{Name: "k, key", Usage: i18n.T("cli.flag.key")},
				// Jump chain flag (Phase 10)
				cli.StringFlag{Name: "J, jump", Usage: i18n.T("cli.flag.jump")},
			},
		},
		{
			Name:   "pull",
			Usage:  i18n.T("cli.cmd.pull"),
			Action: fsPullAction(transferSvc, serverSvc, kr, connectSvc),
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "r, recursive", Usage: i18n.T("cli.flag.recursive")},
				cli.BoolFlag{Name: "e, resume", Usage: i18n.T("cli.flag.resume")},
				cli.BoolFlag{Name: "f, force", Usage: i18n.T("cli.flag.force")},
				cli.BoolFlag{Name: "skip", Usage: i18n.T("cli.flag.skip_existing")},
				cli.BoolFlag{Name: "S, checksum", Usage: i18n.T("cli.flag.verify")},
				cli.IntFlag{Name: "c, concurrency", Value: 3, Usage: i18n.T("cli.flag.concurrency")},
				// Direct connection flags (F12)
				cli.StringFlag{Name: "H, host", Usage: i18n.T("cli.flag.host")},
				cli.StringFlag{Name: "u, user", Usage: i18n.T("cli.flag.user")},
				cli.IntFlag{Name: "p, port", Value: 22, Usage: i18n.T("cli.flag.port")},
				cli.StringFlag{Name: "P, password", Usage: i18n.T("cli.flag.password")},
				cli.StringFlag{Name: "i, identity-file", Usage: i18n.T("cli.flag.identity_file")},
				cli.StringFlag{Name: "k, key", Usage: i18n.T("cli.flag.key")},
				// Jump chain flag (Phase 10)
				cli.StringFlag{Name: "J, jump", Usage: i18n.T("cli.flag.jump")},
			},
		},
		{
			Name: "sftp",
			Usage: i18n.T("cli.cmd.sftp"),
			Action: func(c *cli.Context) error {
				if c.NArg() < 1 {
					fmt.Println(i18n.T("cli.sftp.usage"))
					return nil
				}
				serverName := c.Args()[0]
				server, err := serverSvc.GetServer(serverName)
				if err != nil {
					return fmt.Errorf(i18n.T("cli.sftp.error.server_not_found"), serverName, err)
				}
				// Create SFTP client
				connector := sshinfra.NewConnector()
				sshClient, err := connector.Connect(server)
				if err != nil {
					return fmt.Errorf(i18n.T("cli.sftp.error.ssh_failed"), err)
				}
				sftpClient, err := sftp.NewClient(sshClient,
					sftp.MaxConcurrentRequestsPerFile(64),
					sftp.UseConcurrentReads(true),
					sftp.UseConcurrentWrites(true),
				)
				if err != nil {
					sshClient.Close()
					return fmt.Errorf(i18n.T("cli.sftp.error.sftp_failed"), err)
				}
				defer sftpClient.Close()

				// Interactive loop
				remoteDir := "/"
				localDir, _ := os.Getwd()

				fmt.Println(i18n.T("cli.sftp.connected", map[string]interface{}{"Name": serverName}))
				fmt.Println(i18n.T("cli.sftp.welcome"))

				scanner := bufio.NewScanner(os.Stdin)
				for {
					fmt.Fprintf(os.Stdout, "sftp> ")
					if !scanner.Scan() {
						break
					}
					line := strings.TrimSpace(scanner.Text())
					if line == "" {
						continue
					}

					parts := strings.Fields(line)
					if len(parts) == 0 {
						continue
					}

					cmd := parts[0]
					args := parts[1:]

					switch cmd {
					case "quit", "exit", "bye":
						fmt.Println(i18n.T("cli.sftp.goodbye"))
						return nil
					case "help", "?":
						printSFTPHelp()
					case "pwd":
						fmt.Println(remoteDir)
					case "lpwd":
						fmt.Println(localDir)
					case "ls":
						handleLs(sftpClient, remoteDir, args)
					case "lls":
						handleLls(localDir, args)
					case "cd":
						remoteDir = handleCd(sftpClient, remoteDir, args)
					case "lcd":
						localDir = handleLcd(localDir, args)
					case "get":
						handleGet(transferSvc, server, sftpClient, remoteDir, localDir, args)
					case "put":
						handlePut(transferSvc, server, sftpClient, remoteDir, localDir, args)
					case "mkdir":
						handleMkdir(sftpClient, remoteDir, args)
					case "rmdir":
						handleRmdir(sftpClient, remoteDir, args)
					case "rm":
						handleRm(sftpClient, remoteDir, args)
					case "rename":
						handleRename(sftpClient, remoteDir, args)
					case "ln":
						handleLn(sftpClient, remoteDir, args)
					case "df":
						handleDf(sftpClient, remoteDir)
					case "chmod":
						handleChmod(sftpClient, remoteDir, args)
					case "!":
						handleLocalCmd(args)
					default:
						fmt.Println(i18n.T("cli.sftp.unknown_command", map[string]interface{}{"Cmd": cmd}))
					}
				}
				return nil
			},
			Flags: []cli.Flag{},
		},
	}
}

func fsPushAction(transferSvc *service.TransferService, serverSvc *service.ServerService, kr transferport.KnownServerRecorder, connectSvc *service.ConnectService) func(*cli.Context) error {
		return func(c *cli.Context) error {
			if c.NArg() < 3 {
				return cli.ShowSubcommandHelp(c)
			}

			// Check for direct connection parameters (F12)
			host := c.String("H")
			user := c.String("u")
			port := c.Int("p")
			password := c.String("P")
			identityFile := c.String("i")
			if identityFile == "" {
				identityFile = c.String("k")
			}
			jumpChain := c.String("J")

			var name string
			var localPath string
			var remotePath string

			// If -J is provided, first arg is always the target (server name or user@host)
			if jumpChain != "" {
				name = c.Args()[0]
				localPath = c.Args()[1]
				remotePath = c.Args()[2]
			} else if host != "" {
				name = "__direct__"  // Special marker for direct connection
				localPath = c.Args()[0]
				remotePath = c.Args()[1]
			} else {
				name = c.Args()[0]
				localPath = c.Args()[1]
				remotePath = c.Args()[2]
			}

			opts := service.TransferOptions{
				Recursive:      c.Bool("r"),
				Resume:         c.Bool("e") || c.Bool("resume"),
				Concurrency:    c.Int("c"),
				Progress:       true,
				VerifyChecksum: c.Bool("S") || c.Bool("checksum"),
			}

			if c.Bool("f") || c.Bool("force") {
				opts.Overwrite = "force"
			} else if c.Bool("skip") {
				opts.Overwrite = "skip"
			}

			ctx := context.Background()

			// F14: Glob pattern expansion for local files
			matches, err := filepath.Glob(localPath)
				if err != nil {
					return fmt.Errorf(i18n.T("cli.sftp.error.invalid_glob"), err)
				}

			// Helper: push a single file through the selected connection mode
			pushOne := func(file, destPath string) error {
				switch {
				case jumpChain != "":
					return transferSvc.PushFileWithChain(ctx, name, jumpChain, file, destPath, opts)
				case host != "":
					server := &domain.Server{
						Name: "direct-" + host,
						Host: host,
						Port: port,
						User: user,
						Auth: &domain.Auth{
							Password: password,
							KeyFile:   identityFile,
						},
					}
					return transferSvc.PushFileDirect(ctx, server, file, destPath, opts)
				default:
					return transferSvc.PushFile(ctx, name, file, destPath, opts)
				}
			}

			// Handle glob matches (flatten to target directory)
			hasGlobChars := strings.ContainsAny(localPath, "*?[]")
			if hasGlobChars && len(matches) > 0 {
				// Glob pattern matched multiple files
				fmt.Println(i18n.T("cli.sftp.found_uploading", map[string]interface{}{"Count": len(matches)}))
				for _, file := range matches {
					// Get filename for remote path
					fileName := filepath.Base(file)
					destPath := remotePath
					if !strings.HasSuffix(remotePath, "/") {
						destPath = remotePath + "/" + fileName
					} else {
						destPath = remotePath + fileName
					}

					if err := pushOne(file, destPath); err != nil {
						return fmt.Errorf(i18n.T("cli.sftp.error.push_failed"), file, err)
					}
				}
				fmt.Println(i18n.T("cli.sftp.push_completed"))
				return nil
			} else if hasGlobChars && len(matches) == 0 {
				return fmt.Errorf(i18n.T("cli.sftp.error.no_match"), localPath)
			}

			// Single file transfer
			if err := pushOne(localPath, remotePath); err != nil {
				return fmt.Errorf(i18n.T("cli.sftp.error.push_failed"), localPath, err)
			}

			// 记录直连到 known_servers 表（非跳板直连模式）
			if host != "" && jumpChain == "" && kr != nil {
				recordDirectConnectAsync(kr, host, port, user, domain.ComputeAuthFingerprint(password, identityFile))
			}

			fmt.Println(i18n.T("cli.sftp.push_completed"))
			return nil
		}
	}

	func fsPullAction(transferSvc *service.TransferService, serverSvc *service.ServerService, kr transferport.KnownServerRecorder, connectSvc *service.ConnectService) func(*cli.Context) error {
	return func(c *cli.Context) error {
		if c.NArg() < 2 {
			return cli.ShowSubcommandHelp(c)
		}

		// Check for direct connection parameters (F12)
		host := c.String("H")
		user := c.String("u")
		port := c.Int("p")
		password := c.String("P")
		identityFile := c.String("i")
		if identityFile == "" {
			identityFile = c.String("k")
		}
		jumpChain := c.String("J")

		var name string
		var remotePath string
		var localPath string

		// If -J is provided, first arg is the target (server name or user@host)
		if jumpChain != "" {
			name = c.Args()[0]
			remotePath = c.Args()[1]
			localPath = "."
			if c.NArg() >= 3 {
				localPath = c.Args()[2]
			}
		} else if host != "" {
			name = "__direct__"
			remotePath = c.Args()[0]
			localPath = c.Args()[1]
			if c.NArg() >= 3 {
				localPath = c.Args()[2]
			}
		} else {
			name = c.Args()[0]
			remotePath = c.Args()[1]
			localPath = "."
			if c.NArg() >= 3 {
				localPath = c.Args()[2]
			}
		}

		opts := service.TransferOptions{
			Recursive:      c.Bool("r"),
			Resume:         c.Bool("e") || c.Bool("resume"),
			Concurrency:    c.Int("c"),
			Progress:       true,
			VerifyChecksum: c.Bool("S") || c.Bool("checksum"),
		}

		if c.Bool("f") || c.Bool("force") {
			opts.Overwrite = "force"
		} else if c.Bool("skip") {
			opts.Overwrite = "skip"
		}

		ctx := context.Background()

		// Helper: pull a single file through the selected connection mode
		pullOne := func(remoteFile, localDest string) error {
			switch {
			case jumpChain != "":
				return transferSvc.PullFileWithChain(ctx, name, jumpChain, remoteFile, localDest, opts)
			case host != "":
				server := &domain.Server{
					Name: "direct-" + host,
					Host: host,
					Port: port,
					User: user,
					Auth: &domain.Auth{
						Password: password,
						KeyFile:   identityFile,
					},
				}
				return transferSvc.PullFileDirect(ctx, server, remoteFile, localDest, opts)
			default:
				return transferSvc.PullFile(ctx, name, remoteFile, localDest, opts)
			}
		}

		// F20: Remote glob pattern expansion for pull
		hasGlobChars := strings.ContainsAny(remotePath, "*?[]")
		if hasGlobChars {
			listDir := filepath.Dir(remotePath)
			pattern := filepath.Base(remotePath)

			// List remote directory
			var files []transferport.FileInfo
			var err error
			if host != "" && jumpChain == "" {
				server := &domain.Server{
					Name: "direct-" + host,
					Host: host,
					Port: port,
					User: user,
					Auth: &domain.Auth{
						Password: password,
						KeyFile:   identityFile,
					},
				}
				files, err = transferSvc.ListRemoteDirect(server, listDir)
			} else {
				files, err = transferSvc.ListRemote(name, listDir)
			}
			if err != nil {
				return fmt.Errorf(i18n.T("cli.sftp.error.list_remote_failed"), err)
			}

			// Filter matching files
			var matchedFiles []string
			for _, f := range files {
				if f.IsDir {
					continue
				}
				matched, err := filepath.Match(pattern, f.Name)
				if err != nil {
			return fmt.Errorf(i18n.T("cli.sftp.error.invalid_glob"), err)
				}
				if matched {
					matchedFiles = append(matchedFiles, f.Name)
				}
			}

			if len(matchedFiles) == 0 {
				return fmt.Errorf(i18n.T("cli.sftp.error.no_match"), remotePath)
			}

			fmt.Println(i18n.T("cli.sftp.found_downloading", map[string]interface{}{"Count": len(matchedFiles)}))
			for _, fileName := range matchedFiles {
				fullRemotePath := listDir + "/" + fileName
				localDest := localPath
				if !strings.HasSuffix(localPath, "/") {
					localDest = localPath + "/" + fileName
				} else {
					localDest = localPath + fileName
				}

				if err := pullOne(fullRemotePath, localDest); err != nil {
					return fmt.Errorf(i18n.T("cli.sftp.error.pull_failed"), fileName, err)
				}
			}
			fmt.Println(i18n.T("cli.sftp.pull_completed"))
			return nil
		}

		// Single file transfer
		if err := pullOne(remotePath, localPath); err != nil {
			return fmt.Errorf(i18n.T("cli.sftp.error.pull_failed"), remotePath, err)
		}

		// 记录直连到 known_servers 表（非跳板直连模式）
		if host != "" && jumpChain == "" && kr != nil {
			recordDirectConnectAsync(kr, host, port, user, domain.ComputeAuthFingerprint(password, identityFile))
		}

		fmt.Println(i18n.T("cli.sftp.pull_completed"))
		return nil
	}
}

// registerCompletionHints 为各命令注册 shell 补全函数。
// 为 info/rm/mv/rollback/login/run/bc 提供服务器名补全。
func (a *App) registerCompletionHints() {
	serverNameCommands := []string{"info", "rm", "mv", "rollback", "login", "run", "bc", "keygen", "proxy", "tunnel"}

	for i := range a.cli.Commands {
		cmd := &a.cli.Commands[i]
		for _, name := range serverNameCommands {
			if cmd.Name == name {
				cmd.BashComplete = a.serverNameCompletion
			}
		}
	}
}

// serverNameCompletion 提供服务器名补全。
// 当用户在命令后按 Tab 时，自动补全已保存的服务器名称。
func (a *App) serverNameCompletion(c *cli.Context) {
	if a.serverSvc == nil {
		return
	}

	// 获取所有服务器
	servers, err := a.serverSvc.ListServers()
	if err != nil {
		return
	}

	// 输出所有服务器名（bash 会自动根据已输入内容过滤）
	for group, groupServers := range servers {
		for name := range groupServers {
			fmt.Println(domain.JoinName(group, name))
		}
	}
}

// Run 启动 CLI 应用，解析参数并执行对应命令。
func (a *App) Run(args []string) error {
	return a.cli.Run(args)
}

// setupGlobalFlags 注册全局标志，在所有命令执行前由 beforeAction 处理。
//   - -v/--verbose：开启详细日志输出
//   - -q/--quiet：关闭日志输出
//   - -F/--config：指定配置文件路径
//   - -V/--version：打印版本号
//   - -c/--command：执行远程命令（默认 Action 中使用）
//   - -p/--port、-u/--user、-l/--login：连接参数（v1 兼容，默认 Action 中使用）
//   - -P/--password、-i/--identity-file、-k/--key：认证参数（v1 兼容）
//   - -H/--host：直连主机地址（v1 兼容）
//   - -J/--jump：跳板机链（默认 Action 中使用）
func (a *App) setupGlobalFlags() {
	a.cli.Flags = []cli.Flag{
		cli.BoolFlag{Name: "v, verbose", Usage: i18n.T("cli.flag.verbose")},
		cli.BoolFlag{Name: "q, quiet", Usage: i18n.T("cli.flag.quiet")},
		cli.StringFlag{Name: "F, config", Usage: i18n.T("cli.flag.config")},
		cli.BoolFlag{Name: "V, version", Usage: i18n.T("cli.cmd.version")},
		cli.StringFlag{Name: "c, command", Usage: i18n.T("cli.flag.command")},
		cli.IntFlag{Name: "p, port", Value: 0, Usage: i18n.T("cli.flag.port")},
		cli.StringFlag{Name: "u, user", Usage: i18n.T("cli.flag.user")},
		cli.StringFlag{Name: "l, login", Usage: i18n.T("cli.flag.login")},
		cli.StringFlag{Name: "P, password", Usage: i18n.T("cli.flag.password")},
		cli.StringFlag{Name: "i, identity-file", Usage: i18n.T("cli.flag.identity_file")},
		cli.StringFlag{Name: "k, key", Usage: i18n.T("cli.flag.key")},
		cli.StringFlag{Name: "H, host", Usage: i18n.T("cli.flag.host_address")},
		cli.StringFlag{Name: "J, jump", Usage: i18n.T("cli.flag.jump")},
	}
}

// registerCommands 注册所有子命令，包括内置的 version 命令以及通过扩展方法注册的命令。
func (a *App) registerCommands() {
	a.cli.Commands = []cli.Command{
		{
			Name:   "version",
			Usage:  i18n.T("cli.cmd.version"),
			Action: a.versionAction,
		},
	}
	a.registerServerCommands()
	a.registerConnectCommands()
	a.registerKeygenCommands()
	a.registerProxyCommands()
	a.registerTunnelCommands()
	a.registerSyncCommands()
	a.registerHealthCommands()
	a.registerJumpCommands()
	a.registerUpgradeCommands()
}

// beforeAction 是全局 Before 钩子，在每次命令执行前调用。
// 处理全局标志：-V（打印版本退出）、-v（设置 DEBUG 级别）、-q（关闭日志）。
func (a *App) beforeAction(c *cli.Context) error {
	if c.Bool("version") {
		fmt.Println(a.version)
		return cli.NewExitError("", 0)
	}
	if c.Bool("verbose") {
		log.LogLevel = log.DEBUG
	}
	if c.Bool("quiet") {
		log.LogLevel = log.OFF
	}
	return nil
}

// versionAction 打印版本号并退出。
func (a *App) versionAction(c *cli.Context) error {
	fmt.Println(a.version)
	return nil
}

// defaultAction 是默认 Action，当输入不匹配任何命令时触发。
// 行为与 v1 兼容：自动识别目标（name / user@host / -H host），
// 支持 -c/--command 参数执行远程命令。
// 服务器不存在时，自动给出近似名称建议。
func (a *App) defaultAction(c *cli.Context) error {
	target := c.Args().Get(0)
	host := c.String("host")

	if target == "" && host == "" {
		return cli.ShowAppHelp(c)
	}

	client, directInfo, err := a.loginCore(c)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) && target != "" {
			if suggestion, ok := a.serverSvc.SuggestServer(target); ok {
				return fmt.Errorf(i18n.T("cli.sftp.error.did_you_mean"), suggestion)
			}
		}
		return err
	}
	defer a.connectSvc.Close(client)

	if directInfo != nil {
		a.recordDirectConnect(directInfo.host, directInfo.port, directInfo.user, directInfo.authFingerprint)
	}

	if cmd := c.String("command"); cmd != "" {
		return a.connectSvc.Run(client, cmd)
	}
	return a.connectSvc.Shell(client)
}

// firstNonEmpty 返回参数列表中第一个非空字符串。
// 用于处理 --user/--login 等互为别名的标志。
func firstNonEmpty(strs ...string) string {
	for _, s := range strs {
		if s != "" {
			return s
		}
	}
	return ""
}

// fsSFTPAction handles `assh-fs sftp <server>` command - starts interactive SFTP session.
func fsSFTPAction(transferSvc *service.TransferService, serverSvc *service.ServerService) func(*cli.Context) error {
	return func(c *cli.Context) error {
		if c.NArg() < 1 {
			return cli.ShowSubcommandHelp(c)
		}
		serverName := c.Args()[0]
		return fsInteractiveAction(transferSvc, serverSvc, c, serverName)
	}
}

// fsInteractiveAction starts an interactive SFTP session.
func fsInteractiveAction(transferSvc *service.TransferService, serverSvc *service.ServerService, c *cli.Context, serverName string) error {
	// 获取服务器配置
	server, err := serverSvc.GetServer(serverName)
	if err != nil {
		return fmt.Errorf(i18n.T("cli.sftp.error.server_not_found"), serverName, err)
	}

	// 建立 SFTP 连接
	sftpClient, err := connectSFTP(transferSvc, server)
	if err != nil {
		return fmt.Errorf(i18n.T("cli.sftp.error.sftp_failed"), err)
	}
	defer sftpClient.Close()

	// 初始化状态
	remoteDir := "/"
	localDir, _ := os.Getwd()

	fmt.Println(i18n.T("cli.sftp.connected", map[string]interface{}{"Name": serverName}))
	fmt.Println(i18n.T("cli.sftp.welcome"))

	// 交互式循环
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprintf(os.Stdout, "sftp> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// 解析命令
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		cmd := parts[0]
		args := parts[1:]

		switch cmd {
		case "quit", "exit", "bye":
			fmt.Println(i18n.T("cli.sftp.goodbye"))
			return nil
		case "help", "?":
			printSFTPHelp()
		case "pwd":
			fmt.Println(remoteDir)
		case "lpwd":
			fmt.Println(localDir)
		case "ls":
			handleLs(sftpClient, remoteDir, args)
		case "lls":
			handleLls(localDir, args)
		case "cd":
			remoteDir = handleCd(sftpClient, remoteDir, args)
		case "lcd":
			localDir = handleLcd(localDir, args)
		case "get":
			handleGet(transferSvc, server, sftpClient, remoteDir, localDir, args)
		case "put":
			handlePut(transferSvc, server, sftpClient, remoteDir, localDir, args)
		case "mkdir":
			handleMkdir(sftpClient, remoteDir, args)
		case "rmdir":
			handleRmdir(sftpClient, remoteDir, args)
		case "rm":
			handleRm(sftpClient, remoteDir, args)
		case "rename":
			handleRename(sftpClient, remoteDir, args)
		case "ln":
			handleLn(sftpClient, remoteDir, args)
		case "df":
			handleDf(sftpClient, remoteDir)
		case "chmod":
			handleChmod(sftpClient, remoteDir, args)
		case "!":
			handleLocalCmd(args)
		default:
			fmt.Println(i18n.T("cli.sftp.unknown_command", map[string]interface{}{"Cmd": cmd}))
		}
	}
	return nil
}

func connectSFTP(transferSvc *service.TransferService, server *domain.Server) (*sftp.Client, error) {
	// 创建一个简单的 SSH 客户端来建立 SFTP
	// 这里复用现有的 connector
	connector := sshinfra.NewConnector()
	sshClient, err := connector.Connect(server)
	if err != nil {
		return nil, err
	}

	// 创建 SFTP 客户端
	sftpClient, err := sftp.NewClient(sshClient,
		sftp.MaxConcurrentRequestsPerFile(64),
		sftp.UseConcurrentReads(true),
		sftp.UseConcurrentWrites(true),
	)
	if err != nil {
		sshClient.Close()
		return nil, err
	}
	return sftpClient, nil
}

func printSFTPHelp() {
	fmt.Println(i18n.T("cli.sftp.help.text"))
}

func handleLs(client *sftp.Client, cwd string, args []string) {
	path := cwd
	if len(args) > 0 && args[0] != "-l" {
		path = joinPath(cwd, args[0])
	}

	entries, err := client.ReadDir(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.ls_failed", map[string]interface{}{"Error": err}))
		return
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			fmt.Printf("drwxr-xr-x %5d %s/\n", e.Size(), name)
		} else {
			fmt.Printf("-rw-r--r-- %5d %s\n", e.Size(), name)
		}
	}
}

func handleLls(cwd string, args []string) {
	path := cwd
	if len(args) > 0 {
		path = args[0]
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.lls_failed", map[string]interface{}{"Error": err}))
		return
	}

	for _, e := range entries {
		info, _ := e.Info()
		if e.IsDir() {
			fmt.Printf("drwxr-xr-x %5d %s/\n", info.Size(), e.Name())
		} else {
			fmt.Printf("-rw-r--r-- %5d %s\n", info.Size(), e.Name())
		}
	}
}

func handleCd(client *sftp.Client, cwd string, args []string) string {
	if len(args) == 0 {
		return "/"
	}
	path := joinPath(cwd, args[0])

	// 检查目录是否存在
	_, err := client.Stat(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.cd_failed", map[string]interface{}{"Error": err}))
		return cwd
	}
	return path
}

func handleLcd(cwd string, args []string) string {
	if len(args) == 0 {
		home, _ := os.UserHomeDir()
		return home
	}
	path := args[0]
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	if err := os.Chdir(path); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.lcd_failed", map[string]interface{}{"Error": err}))
		return cwd
	}
	newDir, _ := os.Getwd()
	return newDir
}

func handleGet(transferSvc *service.TransferService, server *domain.Server, client *sftp.Client, remoteDir, localDir string, args []string) {
	if len(args) == 0 {
		fmt.Println(i18n.T("cli.sftp.usage_get"))
		return
	}

	remotePath := joinPath(remoteDir, args[0])
	localPath := args[1]
	if localPath == "" {
		localPath = filepath.Join(localDir, args[0])
	}

	// 使用 transfer service 下载
	opts := service.TransferOptions{Progress: true}
	ctx := context.Background()
	if err := transferSvc.PullFile(ctx, server.Name, remotePath, localPath, opts); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.get_failed", map[string]interface{}{"Error": err}))
	} else {
		fmt.Println(i18n.T("cli.sftp.downloaded", map[string]interface{}{"Remote": remotePath, "Local": localPath}))
	}
}

func handlePut(transferSvc *service.TransferService, server *domain.Server, client *sftp.Client, remoteDir, localDir string, args []string) {
	if len(args) == 0 {
		fmt.Println(i18n.T("cli.sftp.usage_put"))
		return
	}

	localPath := args[0]
	if !filepath.IsAbs(localPath) {
		localPath = filepath.Join(localDir, localPath)
	}

	remotePath := args[1]
	if remotePath == "" {
		remotePath = joinPath(remoteDir, filepath.Base(localPath))
	} else if !filepath.IsAbs(remotePath) {
		remotePath = joinPath(remoteDir, remotePath)
	}

	opts := service.TransferOptions{Progress: true}
	ctx := context.Background()
	if err := transferSvc.PushFile(ctx, server.Name, localPath, remotePath, opts); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.put_failed", map[string]interface{}{"Error": err}))
	} else {
		fmt.Println(i18n.T("cli.sftp.uploaded", map[string]interface{}{"Local": localPath, "Remote": remotePath}))
	}
}

func handleMkdir(client *sftp.Client, cwd string, args []string) {
	if len(args) == 0 {
		fmt.Println(i18n.T("cli.sftp.usage_mkdir"))
		return
	}
	path := joinPath(cwd, args[0])
	if err := client.MkdirAll(path); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.mkdir_failed", map[string]interface{}{"Error": err}))
	} else {
		fmt.Println(i18n.T("cli.sftp.created_dir", map[string]interface{}{"Path": path}))
	}
}

func handleRmdir(client *sftp.Client, cwd string, args []string) {
	if len(args) == 0 {
		fmt.Println(i18n.T("cli.sftp.usage_rmdir"))
		return
	}
	path := joinPath(cwd, args[0])
	if err := client.Remove(path); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.rmdir_failed", map[string]interface{}{"Error": err}))
	} else {
		fmt.Println(i18n.T("cli.sftp.removed_dir", map[string]interface{}{"Path": path}))
	}
}

func handleRm(client *sftp.Client, cwd string, args []string) {
	if len(args) == 0 {
		fmt.Println(i18n.T("cli.sftp.usage_rm"))
		return
	}
	path := joinPath(cwd, args[0])
	if err := client.Remove(path); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.rm_failed", map[string]interface{}{"Error": err}))
	} else {
		fmt.Println(i18n.T("cli.sftp.removed_file", map[string]interface{}{"Path": path}))
	}
}

func handleRename(client *sftp.Client, cwd string, args []string) {
	if len(args) < 2 {
		fmt.Println(i18n.T("cli.sftp.usage_rename"))
		return
	}
	oldPath := joinPath(cwd, args[0])
	newPath := joinPath(cwd, args[1])
	if err := client.Rename(oldPath, newPath); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.rename_failed", map[string]interface{}{"Error": err}))
	} else {
		fmt.Println(i18n.T("cli.sftp.renamed", map[string]interface{}{"Old": oldPath, "New": newPath}))
	}
}

func handleLn(client *sftp.Client, cwd string, args []string) {
	if len(args) < 2 {
		fmt.Println(i18n.T("cli.sftp.usage_ln"))
		return
	}
	target := joinPath(cwd, args[0])
	link := joinPath(cwd, args[1])
	if err := client.Symlink(target, link); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.ln_failed", map[string]interface{}{"Error": err}))
	} else {
		fmt.Println(i18n.T("cli.sftp.created_symlink", map[string]interface{}{"Link": link, "Target": target}))
	}
}

func handleDf(client *sftp.Client, cwd string) {
	// 简化版 df - 显示当前目录信息
	info, err := client.Stat(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.df_failed", map[string]interface{}{"Error": err}))
		return
	}
	fmt.Println(i18n.T("cli.sftp.remote_dir", map[string]interface{}{"Dir": cwd}))
	if info.IsDir() {
		fmt.Println(i18n.T("cli.sftp.type_dir"))
	} else {
		fmt.Println(i18n.T("cli.sftp.type_file"))
	}
}

func handleChmod(client *sftp.Client, cwd string, args []string) {
	if len(args) < 2 {
		fmt.Println(i18n.T("cli.sftp.usage_chmod"))
		return
	}
	mode, err := strconv.ParseUint(args[0], 8, 32)
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.invalid_mode"))
		return
	}
	path := joinPath(cwd, args[1])
	if err := client.Chmod(path, os.FileMode(mode)); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.chmod_failed", map[string]interface{}{"Error": err}))
	} else {
		fmt.Println(i18n.T("cli.sftp.changed_mode", map[string]interface{}{"Path": path}))
	}
}

// allowedLocalCmds 白名单：SFTP 交互模式下允许执行的本地命令
var allowedLocalCmds = map[string]bool{
	"ls":    true,
	"ll":    true,
	"dir":   true,
	"cat":   true,
	"head":  true,
	"tail":  true,
	"pwd":   true,
	"echo":  true,
	"grep":  true,
	"wc":    true,
	"sort":  true,
	"uniq":  true,
	"file":  true,
	"stat":  true,
	"du":    true,
	"df":    true,
	"which": true,
	"env":   true,
	"set":   true,
}

func handleLocalCmd(args []string) {
	if len(args) == 0 {
		return
	}

	cmdName := args[0]
	if !allowedLocalCmds[cmdName] {
		fmt.Fprintln(os.Stderr, i18n.T("cli.sftp.error.not_allowed", map[string]interface{}{"Cmd": cmdName}))
		fmt.Fprint(os.Stderr, i18n.T("cli.sftp.allowed_commands"))
		first := true
		for cmd := range allowedLocalCmds {
			if !first {
				fmt.Fprintf(os.Stderr, ", ")
			}
			fmt.Fprintf(os.Stderr, "%s", cmd)
			first = false
		}
		fmt.Fprintf(os.Stderr, "\n")
		return
	}

	cmd := exec.Command(cmdName, args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func joinPath(cwd, p string) string {
	if p == "" {
		return cwd
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(cwd, p)
}



// init 初始化 CLI 帮助模板和环境目录。
// 在帮助文本末尾添加环境变量说明，并确保数据目录存在。
func init() {
	cli.AppHelpTemplate = fmt.Sprintf(`%s

%s

`, cli.AppHelpTemplate, i18n.T("cli.environment.config_dir"))

	if err := config.EnsureDir(config.DataPath); err != nil {
		log.Warnf("failed to ensure data directory %s: %v", config.DataPath, err)
	}
	if err := os.MkdirAll("/tmp", 0755); err != nil {
		log.Warnf("failed to create /tmp directory: %v", err)
	}
}
