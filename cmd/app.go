// Package cmd 实现 CLI 命令层，基于 urfave/cli 框架。
//
// 本层负责命令注册、参数解析、全局标志处理和用户交互。
// 命令处理逻辑委托给 service 层，遵循"瘦 handler + 胖 service"模式。
package cmd

import (
	"errors"
	"fmt"
	"os"

	"assh/asshc/domain"
	"assh/asshc/service"
	"assh/config"
	"assh/log"

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
