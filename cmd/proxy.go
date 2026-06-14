package cmd

import (
	"fmt"

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
  assh proxy rule reload                       # hot-reload rules
`,
		Flags:  proxyFlags(),
		Action: a.proxyAction,
		Subcommands: []cli.Command{
			{
				Name:  "rule",
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
			},
			{
				Name:      "log",
				Usage:     "View proxy session logs",
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
