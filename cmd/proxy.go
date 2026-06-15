package cmd

import (
	"fmt"

	"assh/asshc/service"
	"assh/internal/i18n"

	"github.com/urfave/cli"
)

// proxyFlags returns the common set of flags for proxy commands.
func proxyFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{Name: "socks", Value: "", Usage: i18n.T("cmd.proxy.flag.socks")},
		cli.StringFlag{Name: "socks5", Value: "", Usage: i18n.T("cmd.proxy.flag.socks5")},
		cli.StringFlag{Name: "http", Value: "", Usage: i18n.T("cmd.proxy.flag.http")},
		cli.StringFlag{Name: "tcp", Value: "", Usage: i18n.T("cmd.proxy.flag.tcp")},
		cli.BoolFlag{Name: "reverse", Usage: i18n.T("cmd.proxy.flag.reverse")},
		cli.StringFlag{Name: "listen", Value: "tcp", Usage: i18n.T("cmd.proxy.flag.listen_type")},
		cli.StringFlag{Name: "ports", Value: "", Usage: i18n.T("cmd.proxy.flag.ports")},
		cli.StringFlag{Name: "auth", Value: "", Usage: i18n.T("cmd.proxy.flag.auth")},
		cli.StringFlag{Name: "autoproxy", Value: "", Usage: i18n.T("cmd.proxy.flag.autoproxy")},
		cli.StringFlag{Name: "log-dir", Value: "", Usage: i18n.T("cmd.proxy.flag.log_dir")},
		cli.BoolFlag{Name: "daemon, d", Usage: i18n.T("cmd.proxy.flag.daemon")},
		cli.StringFlag{Name: "auto-reconnect", Value: "", Usage: i18n.T("cmd.proxy.flag.reconnect")},
		cli.StringFlag{Name: "H, host", Usage: "remote host address (direct connection)"},
		cli.StringFlag{Name: "u, user", Usage: "username (direct connection)"},
		cli.StringFlag{Name: "P, password", Usage: "password (direct connection)"},
		cli.StringFlag{Name: "i, identity-file", Usage: "identity file path (direct connection)"},
		cli.StringFlag{Name: "k, key", Usage: "key file path (alias for -i)"},
		cli.StringFlag{Name: "J, jump", Usage: i18n.T("cmd.proxy.flag.jump")},
	}
}

func (a *App) registerProxyCommands() {
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:      "proxy",
		Usage:     i18n.T("cmd.proxy.description"),
		ArgsUsage: "[server]",
		Description: i18n.T("cmd.proxy.description_long"),
		Flags:  proxyFlags(),
		Action: a.proxyAction,
		Subcommands: []cli.Command{
			{
				Name:  "rule",
				Usage: i18n.T("cmd.proxy.rules.description"),
				Subcommands: []cli.Command{
					{
						Name:   "reload",
						Usage:  i18n.T("cmd.proxy.rules.reload.description"),
						Action: a.proxyRuleReloadAction,
					},
					{
						Name:   "status",
						Usage:  i18n.T("cmd.proxy.rules.status.description"),
						Action: a.proxyRuleStatusAction,
					},
				},
			},
			{
				Name:      "log",
				Usage:     i18n.T("cmd.proxy.log.description"),
				UsageText: "proxy log <session-id>",
				Action:    a.proxyLogAction,
			},
		},
	})
}

func (a *App) proxyAction(c *cli.Context) error {
	host := c.String("H")
	jumpChain := c.String("J")

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

	if host != "" {
		if a.proxySvc == nil {
			return fmt.Errorf("%s", i18n.T("cmd.proxy.error.service_unavailable"))
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
		return fmt.Errorf("%s", i18n.T("cmd.proxy.error.service_unavailable"))
	}
	if jumpChain != "" {
		return a.proxySvc.StartProxyWithChain(serverName, jumpChain, opts)
	}

	return a.proxySvc.StartProxy(serverName, opts)
}

func (a *App) proxyRuleReloadAction(c *cli.Context) error {
	if a.proxySvc == nil {
		return fmt.Errorf("%s", i18n.T("cmd.proxy.error.service_unavailable"))
	}
	return a.proxySvc.ReloadRules()
}

func (a *App) proxyRuleStatusAction(c *cli.Context) error {
	fmt.Println(i18n.T("cmd.proxy.rules.loaded"))
	return nil
}

func (a *App) proxyLogAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return cli.ShowSubcommandHelp(c)
	}
	sessionID := c.Args()[0]
	fmt.Println(i18n.T("cmd.proxy.log.viewer_tbd", map[string]interface{}{"Session": sessionID}))
	return nil
}
