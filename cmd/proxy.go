package cmd

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"assh/asshc/domain"
	"assh/asshc/service"

	"github.com/urfave/cli"
)

// proxyFlags returns the common set of flags for proxy commands.
func proxyFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{Name: "socks", Value: "", Usage: "SOCKS5 proxy listen address (local, default :1080)"},
		cli.StringFlag{Name: "socks5", Value: "", Usage: "alias for --socks (deprecated)"},
		cli.StringFlag{Name: "http", Value: "", Usage: "HTTP CONNECT proxy listen address"},
		cli.StringFlag{Name: "tcp", Value: "", Usage: "TCP proxy listen address"},
		cli.BoolFlag{Name: "reverse", Usage: "reverse proxy mode (target is local)"},
		cli.StringFlag{Name: "listen", Value: "tcp", Usage: "listen type for remote server: tcp|http"},
		cli.StringFlag{Name: "ports", Value: "", Usage: "port mapping rules (e.g. 80,443:80)"},
		cli.StringFlag{Name: "auth", Value: "", Usage: "proxy authentication (user:pass)"},
		cli.StringFlag{Name: "autoproxy", Value: "", Usage: "AutoProxy rule file path"},
		cli.StringFlag{Name: "log-dir", Value: "", Usage: "proxy log directory"},
		cli.BoolFlag{Name: "daemon, d", Usage: "run in background as daemon"},
		cli.StringFlag{Name: "auto-reconnect", Value: "", Usage: "auto reconnect [retries/]interval (e.g. 3/5s)"},
		cli.StringFlag{Name: "H, host", Usage: "remote host address (direct connection)"},
		cli.StringFlag{Name: "u, user", Usage: "username (direct connection)"},
		cli.StringFlag{Name: "P, password", Usage: "password (direct connection)"},
		cli.StringFlag{Name: "i, identity-file", Usage: "identity file path (direct connection)"},
		cli.StringFlag{Name: "k, key", Usage: "key file path (alias for -i)"},
		cli.StringFlag{Name: "J, jump", Usage: "jump host chain (comma-separated): name,user@host[:port]"},
	}
}

func (a *App) registerProxyCommands() {
	// 添加 --save 标志
	flags := proxyFlags()
	flags = append(flags, cli.StringFlag{Name: "save", Value: "", Usage: "save proxy configuration with name"})

	// 注意：proxy 命令不能有 Subcommands，否则 urfave/cli v1 不会解析主命令的 flags。
	// 所有子命令注册为顶级命令：proxy-list, proxy-start, proxy-stop, proxy-rm。
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:      "proxy",
		Usage:     "Start SOCKS5/HTTP CONNECT proxy with Smart Proxy support",
		ArgsUsage: "[server]",
		Description: `Start a proxy through SSH tunnel.

Examples:
  assh proxy myserver                          # SOCKS5 :1080 (default)
  assh proxy myserver --socks5 :1080 --http :8080  # SOCKS5 + HTTP CONNECT
  assh proxy myserver --auth user:pass         # with authentication
  assh proxy myserver --daemon                 # background daemon
  assh proxy myserver --auto-reconnect=3/5s    # with auto-reconnect
  assh proxy -H 192.168.1.1 -u root -P pass   # direct connection
  assh proxy myserver --autoproxy ./list.txt   # with AutoProxy rules
  assh proxy myserver --reverse --ports 80:3000  # reverse proxy
  assh proxy myserver --socks :1080 --save my-proxy  # save and start
  assh proxy-list                              # list saved proxies
  assh proxy-start my-proxy                    # start saved proxy
  assh proxy-stop my-proxy                     # stop running proxy
`,
		Flags: flags,
		Action: a.proxyAction,
	})

	// proxy 子命令注册为顶级命令（避免 urfave/cli v1 的 Subcommands flag 解析 bug）
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:   "proxy-list",
		Usage:  "List saved proxy configurations",
		Action: a.proxyListAction,
	})
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:      "proxy-start",
		Usage:     "Start a saved proxy configuration",
		ArgsUsage: "<name|id>",
		Action:    a.proxyStartAction,
	})
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:      "proxy-stop",
		Usage:     "Stop a running proxy",
		ArgsUsage: "<name|id>",
		Action:    a.proxyStopAction,
	})
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:      "proxy-rm",
		Usage:     "Remove a saved proxy configuration",
		ArgsUsage: "<name|id>",
		Action:    a.proxyRmAction,
	})

	// proxy-rule 子命令单独注册
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:  "proxy-rule",
		Usage: "Manage AutoProxy rules",
		Subcommands: []cli.Command{
			{
				Name:   "reload",
				Usage:  "Reload AutoProxy rules from file",
				Action: a.proxyRuleReloadAction,
			},
			{
				Name:   "status",
				Usage:  "Show AutoProxy rule status",
				Action: a.proxyRuleStatusAction,
			},
		},
	})

	// proxy-log 子命令单独注册
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:      "proxy-log",
		Usage:     "View proxy session logs",
		UsageText: "proxy-log <session-id>",
		Action:    a.proxyLogAction,
	})
}

func (a *App) proxyAction(c *cli.Context) error {
	host := c.String("H")
	jumpChain := c.String("J")
	saveName := c.String("save")

	// 处理 --socks5 别名
	socksAddr := c.String("socks")
	if socksAddr == "" {
		socksAddr = c.String("socks5")
	}

	opts := service.ProxyOptions{
		SOCKS5Addr:    socksAddr,
		HTTPAddr:      c.String("http"),
		TCPAddr:       c.String("tcp"),
		Reverse:       c.Bool("reverse"),
		ListenType:    c.String("listen"),
		Auth:          c.String("auth"),
		Ports:         c.String("ports"),
		RuleFile:      c.String("autoproxy"),
		LogDir:        c.String("log-dir"),
		Daemon:        c.Bool("daemon"),
		AutoReconnect: c.String("auto-reconnect"),
	}

	// 如果指定了 --save，保存配置
	if saveName != "" {
		if a.proxyRepo == nil {
			return fmt.Errorf("proxy repository not available")
		}

		svc := &domain.ProxyService{
			Name:          saveName,
			GroupName:     "",
			ServerName:    c.Args().Get(0),
			Host:          host,
			Port:          c.Int("p"),
			UserName:      c.String("u"),
			JumpChain:     jumpChain,
			ProxyType:     "socks5",
			SOCKS5Addr:    socksAddr,
			HTTPAddr:      c.String("http"),
			TCPAddr:       c.String("tcp"),
			Reverse:       c.Bool("reverse"),
			ListenType:    c.String("listen"),
			Auth:          c.String("auth"),
			RuleFile:      c.String("autoproxy"),
			LogDir:        c.String("log-dir"),
			Daemon:        c.Bool("daemon"),
			AutoReconnect: c.String("auto-reconnect"),
			Enabled:       true,
		}

		// 确定代理类型
		if svc.HTTPAddr != "" {
			svc.ProxyType = "http"
		} else if svc.TCPAddr != "" {
			svc.ProxyType = "tcp"
		}

		if err := a.proxyRepo.Create(svc); err != nil {
			return fmt.Errorf("save proxy failed: %w", err)
		}
		fmt.Printf("Proxy configuration saved as %q (id=%d)\n", saveName, svc.ID)
	}

	if host != "" {
		if a.proxySvc == nil {
			return fmt.Errorf("proxy service not available")
		}

		user := c.String("u")
		if user == "" {
			user = c.String("l")
		}
		port := c.Int("p")
		if port <= 0 {
			port = 22
		}
		password := c.String("P")
		keyFile := c.String("i")
		if keyFile == "" {
			keyFile = c.String("k")
		}

		return a.proxySvc.StartDirectProxy(host, port, user, password, keyFile, opts)
	}

	if c.NArg() < 1 {
		return cli.ShowSubcommandHelp(c)
	}

	serverName := c.Args()[0]

	if a.proxySvc == nil {
		return fmt.Errorf("proxy service not available")
	}

	// 跳板链模式
	if jumpChain != "" {
		return a.proxySvc.StartProxyWithChain(serverName, jumpChain, opts)
	}

	return a.proxySvc.StartProxy(serverName, opts)
}

// resolveProxyService 根据名字或 ID 解析代理服务
func (a *App) resolveProxyService(nameOrID string) (*domain.ProxyService, error) {
	if a.proxyRepo == nil {
		return nil, fmt.Errorf("proxy repository not available")
	}

	// 尝试按 ID 解析
	if id, err := strconv.ParseInt(nameOrID, 10, 64); err == nil {
		return a.proxyRepo.GetByID(id)
	}

	// 按名字解析
	return a.proxyRepo.GetByName("", nameOrID)
}

func (a *App) proxyListAction(c *cli.Context) error {
	if a.proxyRepo == nil {
		return fmt.Errorf("proxy repository not available")
	}

	services, err := a.proxyRepo.List("")
	if err != nil {
		return fmt.Errorf("list proxies failed: %w", err)
	}

	if len(services) == 0 {
		fmt.Println("No saved proxy configurations")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tNAME\tSERVER\tTYPE\tLISTEN\tSTATUS\n")
	for _, svc := range services {
		status := "enabled"
		if !svc.Enabled {
			status = "disabled"
		}
		listen := svc.SOCKS5Addr
		if svc.HTTPAddr != "" {
			listen = svc.HTTPAddr
		} else if svc.TCPAddr != "" {
			listen = svc.TCPAddr
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n", svc.ID, svc.Name, svc.ServerName, svc.ProxyType, listen, status)
	}
	w.Flush()
	return nil
}

func (a *App) proxyStartAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return cli.ShowSubcommandHelp(c)
	}

	svc, err := a.resolveProxyService(c.Args()[0])
	if err != nil {
		return fmt.Errorf("proxy not found: %w", err)
	}

	if a.proxySvc == nil {
		return fmt.Errorf("proxy service not available")
	}

	opts := service.ProxyOptions{
		SOCKS5Addr:    svc.SOCKS5Addr,
		HTTPAddr:      svc.HTTPAddr,
		TCPAddr:       svc.TCPAddr,
		Reverse:       svc.Reverse,
		ListenType:    svc.ListenType,
		Auth:          svc.Auth,
		RuleFile:      svc.RuleFile,
		LogDir:        svc.LogDir,
		Daemon:        svc.Daemon,
		AutoReconnect: svc.AutoReconnect,
	}

	// 记录使用
	if a.proxyRepo != nil {
		_ = a.proxyRepo.RecordUsage(svc.ID)
	}

	if svc.Host != "" {
		return a.proxySvc.StartDirectProxy(svc.Host, svc.Port, svc.UserName, "", "", opts)
	}

	if svc.JumpChain != "" {
		return a.proxySvc.StartProxyWithChain(svc.ServerName, svc.JumpChain, opts)
	}

	return a.proxySvc.StartProxy(svc.ServerName, opts)
}

func (a *App) proxyStopAction(c *cli.Context) error {
	if a.proxySvc == nil {
		return fmt.Errorf("proxy service not available")
	}
	return a.proxySvc.StopProxy()
}

func (a *App) proxyRmAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return cli.ShowSubcommandHelp(c)
	}

	svc, err := a.resolveProxyService(c.Args()[0])
	if err != nil {
		return fmt.Errorf("proxy not found: %w", err)
	}

	if err := a.proxyRepo.Delete(svc.ID); err != nil {
		return fmt.Errorf("delete proxy failed: %w", err)
	}

	fmt.Printf("Proxy %q (id=%d) removed\n", svc.Name, svc.ID)
	return nil
}

func (a *App) proxyRuleReloadAction(c *cli.Context) error {
	if a.proxySvc == nil {
		return fmt.Errorf("proxy service not available")
	}
	return a.proxySvc.ReloadRules()
}

func (a *App) proxyRuleStatusAction(c *cli.Context) error {
	fmt.Println("AutoProxy rules loaded (status check via --autoproxy)")
	return nil
}

func (a *App) proxyLogAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return cli.ShowSubcommandHelp(c)
	}
	sessionID := c.Args()[0]
	fmt.Printf("Session log for %s: (log viewer TBD)\n", sessionID)
	return nil
}
