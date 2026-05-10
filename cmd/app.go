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
	"assh/asshc/service"
	"assh/config"
	"assh/log"

	"github.com/pkg/sftp"
	"github.com/urfave/cli"
)

// App 封装 CLI 应用的核心结构，持有 service 依赖并通过 urfave/cli 框架注册命令。
type App struct {
	cli        *cli.App                   // urfave/cli 应用实例
	version    string                     // 版本号（编译时注入）
	build      string                     // 构建信息（编译时注入）
	connectSvc *service.ConnectService    // SSH 连接服务
	serverSvc  *service.ServerService     // 服务器配置管理服务
}

// NewApp 创建 CLI 应用，注入所有 service 依赖。
// 注册全局标志（-v/-q/-F/-V）和子命令（server/login/run/bc）。
func NewApp(version, build string, connectSvc *service.ConnectService, serverSvc *service.ServerService) *App {
	app := cli.NewApp()
	app.Name = "ASSH - An SSH Client"
	app.Usage = "An SSH Client"
	app.EnableBashCompletion = true
	app.HideVersion = true

	a := &App{
		cli:        app,
		version:    version,
		build:      build,
		connectSvc: connectSvc,
		serverSvc:  serverSvc,
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
func NewFSApp(version, build string, transferSvc *service.TransferService, serverSvc *service.ServerService) *App {
	app := cli.NewApp()
	app.Name = "ASSH-FS - SFTP File Transfer"
	app.Usage = "SFTP File Transfer Client"
	app.EnableBashCompletion = true
	app.HideVersion = true

	a := &App{
		cli:       app,
		version:   version,
		build:     build,
		serverSvc: serverSvc,
	}
	app.Before = a.beforeAction
	a.setupGlobalFlags()
	a.registerFSCommands(transferSvc, serverSvc)

	return a
}

// registerFSCommands registers file transfer commands.
func (a *App) registerFSCommands(transferSvc *service.TransferService, serverSvc *service.ServerService) {
	a.cli.Commands = []cli.Command{
		{
			Name:   "version",
			Usage:  "Print version information",
			Action: a.versionAction,
		},
		{
			Name:   "push",
			Usage:  "Push local file to remote server",
			Action: fsPushAction(transferSvc, serverSvc),
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "r, recursive", Usage: "upload directory recursively"},
				cli.BoolFlag{Name: "e, resume", Usage: "resume interrupted transfer"},
				cli.BoolFlag{Name: "f, force", Usage: "force overwrite"},
				cli.BoolFlag{Name: "skip", Usage: "skip existing files"},
				cli.BoolFlag{Name: "S, checksum", Usage: "verify SHA256 checksum after transfer"},
				cli.IntFlag{Name: "c, concurrency", Value: 3, Usage: "number of concurrent transfers"},
				// Direct connection flags (F12)
				cli.StringFlag{Name: "H, host", Usage: "remote host address (direct connection)"},
				cli.StringFlag{Name: "u, user", Usage: "username (direct connection)"},
				cli.IntFlag{Name: "p, port", Value: 22, Usage: "SSH port (direct connection)"},
				cli.StringFlag{Name: "P, password", Usage: "password (direct connection)"},
				cli.StringFlag{Name: "i, identity-file", Usage: "identity file path (direct connection)"},
				cli.StringFlag{Name: "k, key", Usage: "key file path (alias for -i)"},
			},
		},
		{
			Name:   "pull",
			Usage:  "Pull remote file to local",
			Action: fsPullAction(transferSvc, serverSvc),
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "r, recursive", Usage: "download directory recursively"},
				cli.BoolFlag{Name: "e, resume", Usage: "resume interrupted transfer"},
				cli.BoolFlag{Name: "f, force", Usage: "force overwrite"},
				cli.BoolFlag{Name: "skip", Usage: "skip existing files"},
				cli.BoolFlag{Name: "S, checksum", Usage: "verify SHA256 checksum after transfer"},
				cli.IntFlag{Name: "c, concurrency", Value: 3, Usage: "number of concurrent transfers"},
				// Direct connection flags (F12)
				cli.StringFlag{Name: "H, host", Usage: "remote host address (direct connection)"},
				cli.StringFlag{Name: "u, user", Usage: "username (direct connection)"},
				cli.IntFlag{Name: "p, port", Value: 22, Usage: "SSH port (direct connection)"},
				cli.StringFlag{Name: "P, password", Usage: "password (direct connection)"},
				cli.StringFlag{Name: "i, identity-file", Usage: "identity file path (direct connection)"},
				cli.StringFlag{Name: "k, key", Usage: "key file path (alias for -i)"},
			},
		},
		{
			Name: "sftp",
			Usage: "Start interactive SFTP session",
			Action: func(c *cli.Context) error {
				if c.NArg() < 1 {
					fmt.Println("Usage: assh-fs sftp <server>")
					return nil
				}
				serverName := c.Args()[0]
				server, err := serverSvc.GetServer(serverName)
				if err != nil {
					return fmt.Errorf("server %q not found: %w", serverName, err)
				}
				// Create SFTP client
				connector := sshinfra.NewConnector()
				sshClient, err := connector.Connect(server)
				if err != nil {
					return fmt.Errorf("SSH connection failed: %w", err)
				}
				sftpClient, err := sftp.NewClient(sshClient,
					sftp.MaxConcurrentRequestsPerFile(64),
					sftp.UseConcurrentReads(true),
					sftp.UseConcurrentWrites(true),
				)
				if err != nil {
					sshClient.Close()
					return fmt.Errorf("SFTP connection failed: %w", err)
				}
				defer sftpClient.Close()

				// Interactive loop
				remoteDir := "/"
				localDir, _ := os.Getwd()

				fmt.Printf("Connected to %s\n", serverName)
				fmt.Println("Type 'help' for available commands, 'quit' to exit.")

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
						fmt.Println("Goodbye!")
						return nil
					case "help", "?":
						fmt.Println(`Available commands:
  ls [path]           - list remote directory
  cd <path>           - change remote directory
  pwd                 - print working directory
  get <remote> [local] - download file
  put <local> [remote] - upload file
  mkdir <path>        - create remote directory
  rmdir <path>        - remove remote directory
  rm <path>           - remove remote file
  lls                 - list local directory
  lcd <path>          - change local directory
  lpwd                - print local working directory
  !<command>          - execute local command
  help                - show this help
  quit                - exit`)
					case "pwd":
						fmt.Println(remoteDir)
					case "lpwd":
						fmt.Println(localDir)
					case "ls":
						handleSftpLs(sftpClient, remoteDir, args)
					case "lls":
						handleSftpLls(localDir, args)
					case "cd":
						remoteDir = handleSftpCd(sftpClient, remoteDir, args)
					case "lcd":
						localDir = handleSftpLcd(localDir, args)
					case "get":
						handleSftpGet(transferSvc, server, sftpClient, remoteDir, localDir, args)
					case "put":
						handleSftpPut(transferSvc, server, sftpClient, remoteDir, localDir, args)
					case "mkdir":
						handleSftpMkdir(sftpClient, remoteDir, args)
					case "rmdir":
						handleSftpRmdir(sftpClient, remoteDir, args)
					case "rm":
						handleSftpRm(sftpClient, remoteDir, args)
					case "!":
						handleSftpLocalCmd(args)
					default:
						fmt.Printf("Unknown command: %s\n", cmd)
					}
				}
				return nil
			},
			Flags: []cli.Flag{},
		},
	}
}

func fsPushAction(transferSvc *service.TransferService, serverSvc *service.ServerService) func(*cli.Context) error {
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

		var name string
		var localPath string
		var remotePath string

		// If -H is provided, use direct connection mode
		if host != "" {
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

		// Direct connection mode
		if host != "" {
			// Create a temporary server config for direct connection
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
			if err := transferSvc.PushFileDirect(ctx, server, localPath, remotePath, opts); err != nil {
				return fmt.Errorf("push failed: %w", err)
			}
		} else {
			// Normal mode - use server name lookup
			if err := transferSvc.PushFile(ctx, name, localPath, remotePath, opts); err != nil {
				return fmt.Errorf("push failed: %w", err)
			}
		}

		fmt.Println("push completed")
		return nil
	}
}

func fsPullAction(transferSvc *service.TransferService, serverSvc *service.ServerService) func(*cli.Context) error {
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

		var name string
		var remotePath string
		var localPath string

		// If -H is provided, use direct connection mode
		if host != "" {
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

		// Direct connection mode
		if host != "" {
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
			if err := transferSvc.PullFileDirect(ctx, server, remotePath, localPath, opts); err != nil {
				return fmt.Errorf("pull failed: %w", err)
			}
} else {
			if err := transferSvc.PullFile(ctx, name, remotePath, localPath, opts); err != nil {
				return fmt.Errorf("pull failed: %w", err)
			}
		}

		fmt.Println("pull completed")
		return nil
	}
}

// registerCompletionHints 为各命令注册 shell 补全函数。
// 为 info/rm/mv/rollback/login/run/bc 提供服务器名补全。
func (a *App) registerCompletionHints() {
	serverNameCommands := []string{"info", "rm", "mv", "rollback", "login", "run", "bc"}

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
func (a *App) setupGlobalFlags() {
	a.cli.Flags = []cli.Flag{
		cli.BoolFlag{Name: "v, verbose", Usage: "verbose output"},
		cli.BoolFlag{Name: "q, quiet", Usage: "quiet mode"},
		cli.StringFlag{Name: "F, config", Usage: "config file path (default: ~/.assh/v2/assh.yml)"},
		cli.BoolFlag{Name: "V, version", Usage: "print version information"},
		cli.StringFlag{Name: "c, command", Usage: "run command on remote server"},
		cli.IntFlag{Name: "p, port", Value: 0, Usage: "port"},
		cli.StringFlag{Name: "u, user", Usage: "username"},
		cli.StringFlag{Name: "l, login", Usage: "username (same as --user)"},
		cli.StringFlag{Name: "P, password", Usage: "password"},
		cli.StringFlag{Name: "i, identity-file", Usage: "identity file path"},
		cli.StringFlag{Name: "k, key", Usage: "key file path (same as --identity-file)"},
		cli.StringFlag{Name: "H, host", Usage: "host address"},
	}
}

// registerCommands 注册所有子命令，包括内置的 version 命令以及通过扩展方法注册的命令。
func (a *App) registerCommands() {
	a.cli.Commands = []cli.Command{
		{
			Name:   "version",
			Usage:  "Print version information",
			Action: a.versionAction,
		},
	}
	a.registerServerCommands()
	a.registerConnectCommands()
}

// beforeAction 是全局 Before 钩子，在每次命令执行前调用。
// 处理全局标志：-V（打印版本退出）、-v（设置 DEBUG 级别）、-q（关闭日志）。
func (a *App) beforeAction(c *cli.Context) error {
	if c.Bool("version") {
		fmt.Println(a.version)
		os.Exit(0)
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

	client, err := a.loginCore(c)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) && target != "" {
			if suggestion, ok := a.serverSvc.SuggestServer(target); ok {
				return fmt.Errorf("server not found, did you mean [%s]?", suggestion)
			}
		}
		return err
	}
	defer a.connectSvc.Close(client)

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
		return fmt.Errorf("server %q not found: %w", serverName, err)
	}

	// 建立 SFTP 连接
	sftpClient, err := connectSFTP(transferSvc, server)
	if err != nil {
		return fmt.Errorf("SFTP connection failed: %w", err)
	}
	defer sftpClient.Close()

	// 初始化状态
	remoteDir := "/"
	localDir, _ := os.Getwd()

	fmt.Printf("Connected to %s\n", serverName)
	fmt.Println("Type 'help' for available commands, 'quit' to exit.")

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
			fmt.Println("Goodbye!")
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
			fmt.Printf("Unknown command: %s\n", cmd)
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
	fmt.Println(`Available commands:
  ls [path]           - list remote directory
  cd <path>           - change remote directory
  pwd                 - print working directory
  get <remote> [local] - download file
  put <local> [remote] - upload file
  mkdir <path>        - create remote directory
  rmdir <path>        - remove remote directory
  rm <path>           - remove remote file
  rename <old> <new>  - rename file
  ln <src> <target>   - create symbolic link
  chmod <mode> <path> - change permissions
  df                  - show disk usage
  lls                 - list local directory
  lcd <path>          - change local directory
  lpwd                - print local working directory
  !<command>          - execute local command
  help                - show this help
  quit                - exit`)
}

func handleLs(client *sftp.Client, cwd string, args []string) {
	path := cwd
	if len(args) > 0 && args[0] != "-l" {
		path = joinPath(cwd, args[0])
	}

	entries, err := client.ReadDir(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ls failed: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "lls failed: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "cd failed: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "lcd failed: %v\n", err)
		return cwd
	}
	newDir, _ := os.Getwd()
	return newDir
}

func handleGet(transferSvc *service.TransferService, server *domain.Server, client *sftp.Client, remoteDir, localDir string, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: get <remote> [local]")
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
		fmt.Fprintf(os.Stderr, "get failed: %v\n", err)
	} else {
		fmt.Printf("Downloaded: %s -> %s\n", remotePath, localPath)
	}
}

func handlePut(transferSvc *service.TransferService, server *domain.Server, client *sftp.Client, remoteDir, localDir string, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: put <local> [remote]")
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
		fmt.Fprintf(os.Stderr, "put failed: %v\n", err)
	} else {
		fmt.Printf("Uploaded: %s -> %s\n", localPath, remotePath)
	}
}

func handleMkdir(client *sftp.Client, cwd string, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mkdir <path>")
		return
	}
	path := joinPath(cwd, args[0])
	if err := client.MkdirAll(path); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir failed: %v\n", err)
	} else {
		fmt.Printf("Created directory: %s\n", path)
	}
}

func handleRmdir(client *sftp.Client, cwd string, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: rmdir <path>")
		return
	}
	path := joinPath(cwd, args[0])
	if err := client.Remove(path); err != nil {
		fmt.Fprintf(os.Stderr, "rmdir failed: %v\n", err)
	} else {
		fmt.Printf("Removed directory: %s\n", path)
	}
}

func handleRm(client *sftp.Client, cwd string, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: rm <path>")
		return
	}
	path := joinPath(cwd, args[0])
	if err := client.Remove(path); err != nil {
		fmt.Fprintf(os.Stderr, "rm failed: %v\n", err)
	} else {
		fmt.Printf("Removed file: %s\n", path)
	}
}

func handleRename(client *sftp.Client, cwd string, args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: rename <old> <new>")
		return
	}
	oldPath := joinPath(cwd, args[0])
	newPath := joinPath(cwd, args[1])
	if err := client.Rename(oldPath, newPath); err != nil {
		fmt.Fprintf(os.Stderr, "rename failed: %v\n", err)
	} else {
		fmt.Printf("Renamed: %s -> %s\n", oldPath, newPath)
	}
}

func handleLn(client *sftp.Client, cwd string, args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: ln <target> <link>")
		return
	}
	target := joinPath(cwd, args[0])
	link := joinPath(cwd, args[1])
	if err := client.Symlink(target, link); err != nil {
		fmt.Fprintf(os.Stderr, "ln failed: %v\n", err)
	} else {
		fmt.Printf("Created symlink: %s -> %s\n", link, target)
	}
}

func handleDf(client *sftp.Client, cwd string) {
	// 简化版 df - 显示当前目录信息
	info, err := client.Stat(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "df failed: %v\n", err)
		return
	}
	fmt.Printf("Remote directory: %s\n", cwd)
	fmt.Printf("Type: %s\n", map[bool]string{true: "Directory", false: "File"}[info.IsDir()])
}

func handleChmod(client *sftp.Client, cwd string, args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: chmod <mode> <path>")
		return
	}
	mode, err := strconv.ParseUint(args[0], 8, 32)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chmod: invalid mode\n")
		return
	}
	path := joinPath(cwd, args[1])
	if err := client.Chmod(path, os.FileMode(mode)); err != nil {
		fmt.Fprintf(os.Stderr, "chmod failed: %v\n", err)
	} else {
		fmt.Printf("Changed mode: %s\n", path)
	}
}

func handleLocalCmd(args []string) {
	if len(args) == 0 {
		return
	}
	cmd := exec.Command(args[0], args[1:]...)
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

// SFTP interactive helper functions
func handleSftpLs(client *sftp.Client, cwd string, args []string) {
	path := cwd
	if len(args) > 0 {
		path = joinPath(cwd, args[0])
	}
	entries, err := client.ReadDir(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ls failed: %v\n", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			fmt.Printf("drwxr-xr-x %5d %s/\n", e.Size(), e.Name())
		} else {
			fmt.Printf("-rw-r--r-- %5d %s\n", e.Size(), e.Name())
		}
	}
}

func handleSftpLls(cwd string, args []string) {
	path := cwd
	if len(args) > 0 {
		path = args[0]
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lls failed: %v\n", err)
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

func handleSftpCd(client *sftp.Client, cwd string, args []string) string {
	if len(args) == 0 {
		return "/"
	}
	path := joinPath(cwd, args[0])
	_, err := client.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cd failed: %v\n", err)
		return cwd
	}
	return path
}

func handleSftpLcd(cwd string, args []string) string {
	if len(args) == 0 {
		home, _ := os.UserHomeDir()
		return home
	}
	path := args[0]
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	if err := os.Chdir(path); err != nil {
		fmt.Fprintf(os.Stderr, "lcd failed: %v\n", err)
		return cwd
	}
	newDir, _ := os.Getwd()
	return newDir
}

func handleSftpGet(transferSvc *service.TransferService, server *domain.Server, client *sftp.Client, remoteDir, localDir string, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: get <remote> [local]")
		return
	}
	remotePath := joinPath(remoteDir, args[0])
	localPath := args[1]
	if localPath == "" {
		localPath = filepath.Join(localDir, args[0])
	}
	opts := service.TransferOptions{Progress: true}
	ctx := context.Background()
	if err := transferSvc.PullFile(ctx, server.Name, remotePath, localPath, opts); err != nil {
		fmt.Fprintf(os.Stderr, "get failed: %v\n", err)
	} else {
		fmt.Printf("Downloaded: %s -> %s\n", remotePath, localPath)
	}
}

func handleSftpPut(transferSvc *service.TransferService, server *domain.Server, client *sftp.Client, remoteDir, localDir string, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: put <local> [remote]")
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
		fmt.Fprintf(os.Stderr, "put failed: %v\n", err)
	} else {
		fmt.Printf("Uploaded: %s -> %s\n", localPath, remotePath)
	}
}

func handleSftpMkdir(client *sftp.Client, cwd string, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mkdir <path>")
		return
	}
	path := joinPath(cwd, args[0])
	if err := client.MkdirAll(path); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir failed: %v\n", err)
	} else {
		fmt.Printf("Created directory: %s\n", path)
	}
}

func handleSftpRmdir(client *sftp.Client, cwd string, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: rmdir <path>")
		return
	}
	path := joinPath(cwd, args[0])
	if err := client.Remove(path); err != nil {
		fmt.Fprintf(os.Stderr, "rmdir failed: %v\n", err)
	} else {
		fmt.Printf("Removed directory: %s\n", path)
	}
}

func handleSftpRm(client *sftp.Client, cwd string, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: rm <path>")
		return
	}
	path := joinPath(cwd, args[0])
	if err := client.Remove(path); err != nil {
		fmt.Fprintf(os.Stderr, "rm failed: %v\n", err)
	} else {
		fmt.Printf("Removed file: %s\n", path)
	}
}

func handleSftpLocalCmd(args []string) {
	if len(args) == 0 {
		return
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// init 初始化 CLI 帮助模板和环境目录。
// 在帮助文本末尾添加环境变量说明，并确保数据目录存在。
func init() {
	cli.AppHelpTemplate = fmt.Sprintf(`%s

ENVIRONMENT:
   ASSH_CONFIG_DIR   config directory (default: ~/.assh/v2)

`, cli.AppHelpTemplate)

	_ = config.EnsureDir(config.DataPath)
	_ = os.MkdirAll("/tmp", 0755)
}
