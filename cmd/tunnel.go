package cmd

import (
	"fmt"
	"strings"

	"assh/asshc/service"
	"assh/internal/i18n"

	"github.com/urfave/cli"
)

func (a *App) registerTunnelCommands() {
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:      "tunnel",
		Usage:     i18n.T("cmd.tunnel.description"),
		ArgsUsage: "<server> [-L/-R spec] or stop/list",
		Description: i18n.T("cmd.tunnel.description_long"),
		Flags: []cli.Flag{
			cli.StringSliceFlag{Name: "L", Usage: i18n.T("cmd.tunnel.flag.local")},
			cli.StringSliceFlag{Name: "R", Usage: i18n.T("cmd.tunnel.flag.remote")},
			cli.StringFlag{Name: "listen", Value: "tcp", Usage: i18n.T("cmd.tunnel.flag.listen_type")},
			cli.BoolFlag{Name: "daemon, d", Usage: i18n.T("cmd.tunnel.flag.daemon")},
			cli.StringFlag{Name: "auto-reconnect", Usage: i18n.T("cmd.tunnel.flag.reconnect")},
			cli.StringFlag{Name: "J, jump", Usage: i18n.T("cmd.tunnel.flag.jump")},
		},
		Action: a.tunnelAction,
		Subcommands: []cli.Command{
			{
				Name:      "stop",
				Usage:     i18n.T("cmd.tunnel.stop.description"),
				ArgsUsage: "<id>",
				Action:    a.tunnelStopAction,
			},
			{
				Name:   "list",
				Usage:  i18n.T("cmd.tunnel.list.description"),
				Action: a.tunnelListAction,
			},
		},
	})
}

func (a *App) tunnelAction(c *cli.Context) error {
	if a.proxySvc == nil {
		return fmt.Errorf("%s", i18n.T("cmd.proxy.error.service_unavailable"))
	}

	if c.NArg() < 1 {
		return cli.ShowSubcommandHelp(c)
	}

	serverName := c.Args()[0]
	jumpChain := c.String("J")
	listenType := c.String("listen")

	localForwards := c.StringSlice("L")
	remoteForwards := c.StringSlice("R")

	if len(localForwards) == 0 && len(remoteForwards) == 0 {
		return fmt.Errorf("%s", i18n.T("cmd.tunnel.error.spec_required"))
	}

	var specs []service.ForwardSpec

	for _, lf := range localForwards {
		localAddr, remoteAddr := parseForwardSpec(lf)
		if localAddr == "" {
			return fmt.Errorf(i18n.T("cmd.tunnel.error.invalid_local"), lf)
		}
		specs = append(specs, service.ForwardSpec{
			LocalAddr:  localAddr,
			RemoteAddr: remoteAddr,
			Reverse:    false,
			ListenType: listenType,
		})
	}

	for _, rf := range remoteForwards {
		remoteAddr, localAddr := parseForwardSpec(rf)
		if remoteAddr == "" {
			return fmt.Errorf(i18n.T("cmd.tunnel.error.invalid_remote"), rf)
		}
		specs = append(specs, service.ForwardSpec{
			LocalAddr:  localAddr,
			RemoteAddr: remoteAddr,
			Reverse:    true,
			ListenType: listenType,
		})
	}

	opts := service.ProxyOptions{
		Daemon:        c.Bool("daemon"),
		AutoReconnect: c.String("auto-reconnect"),
		ListenType:    listenType,
	}

	if jumpChain != "" {
		return a.proxySvc.StartTunnelWithChain(serverName, jumpChain, specs, opts)
	}

	return a.proxySvc.StartTunnel(serverName, specs, opts)
}

func (a *App) tunnelStopAction(c *cli.Context) error {
	if a.proxySvc == nil {
		return fmt.Errorf("%s", i18n.T("cmd.proxy.error.service_unavailable"))
	}

	if c.NArg() < 1 {
		return cli.ShowSubcommandHelp(c)
	}

	id := c.Args()[0]
	return a.proxySvc.StopTunnel(id)
}

func (a *App) tunnelListAction(c *cli.Context) error {
	if a.proxySvc == nil {
		return fmt.Errorf("%s", i18n.T("cmd.proxy.error.service_unavailable"))
	}

	tunnels := a.proxySvc.ListTunnels()
	if len(tunnels) == 0 {
		fmt.Println(i18n.T("cmd.tunnel.list.empty"))
		return nil
	}

	fmt.Printf("%-10s  %-6s  %-30s  %-30s\n",
		i18n.T("cmd.tunnel.list.header.id"),
		i18n.T("cmd.tunnel.list.header.type"),
		i18n.T("cmd.tunnel.list.header.local"),
		i18n.T("cmd.tunnel.list.header.remote"))
	fmt.Println(strings.Repeat("-", 80))
	for _, t := range tunnels {
		fmt.Printf("%-10s  %-6s  %-30s  %-30s\n", t.ID(), t.Type(), t.LocalAddr(), t.RemoteAddr())
	}
	fmt.Printf("\n%s\n", i18n.T("cmd.tunnel.list.total", map[string]interface{}{"Count": len(tunnels)}))
	return nil
}

func parseForwardSpec(spec string) (localAddr, remoteAddr string) {
	parts := strings.Split(spec, ":")

	switch len(parts) {
	case 3:
		// port:host:hostport
		host := parts[1]
		if host == "" {
			host = "127.0.0.1" // 自动补全空 host
		}
		return fmt.Sprintf("127.0.0.1:%s", parts[0]), fmt.Sprintf("%s:%s", host, parts[2])
	case 4:
		// bind:port:host:hostport
		host := parts[2]
		if host == "" {
			host = "127.0.0.1" // 自动补全空 host
		}
		return fmt.Sprintf("%s:%s", parts[0], parts[1]), fmt.Sprintf("%s:%s", host, parts[3])
	default:
		return "", ""
	}
}
