package cmd

import (
	"fmt"
	"strings"

	"assh/asshc/service"

	"github.com/urfave/cli"
)

func (a *App) registerTunnelCommands() {
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:      "tunnel",
		Usage:     "Manage SSH port forwarding tunnels",
		ArgsUsage: "<server> [-L/-R spec] or stop/list",
		Description: `Start SSH port forwarding tunnels.

Examples:
  assh tunnel myserver -L 8080:localhost:80        # local forward
  assh tunnel myserver -R 8080:localhost:80        # remote forward
  assh tunnel myserver -L 80:web:80 -L 443:web:443  # multiple forwards
  assh tunnel myserver -L 8080:localhost:80 --daemon  # background
  assh tunnel stop <id>                            # stop tunnel
  assh tunnel list                                 # list tunnels
`,
		Flags: []cli.Flag{
			cli.StringSliceFlag{Name: "L", Usage: "local forward spec [bind_addr:]port:host:hostport"},
			cli.StringSliceFlag{Name: "R", Usage: "remote forward spec [bind_addr:]port:host:hostport"},
			cli.StringFlag{Name: "listen", Value: "tcp", Usage: "listen type: tcp|http"},
			cli.BoolFlag{Name: "daemon, d", Usage: "run in background as daemon"},
			cli.StringFlag{Name: "auto-reconnect", Usage: "auto reconnect [retries/]interval"},
			cli.StringFlag{Name: "J, jump", Usage: "jump host chain (comma-separated): name,user@host[:port]"},
		},
		Action: a.tunnelAction,
		Subcommands: []cli.Command{
			{
				Name:      "stop",
				Usage:     "Stop a running tunnel",
				ArgsUsage: "<id>",
				Action:    a.tunnelStopAction,
			},
			{
				Name:   "list",
				Usage:  "List all running tunnels",
				Action: a.tunnelListAction,
			},
		},
	})
}

func (a *App) tunnelAction(c *cli.Context) error {
	if a.proxySvc == nil {
		return fmt.Errorf("proxy service not available")
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
		return fmt.Errorf("at least one -L or -R flag is required")
	}

	var specs []service.ForwardSpec

	for _, lf := range localForwards {
		localAddr, remoteAddr := parseForwardSpec(lf)
		if localAddr == "" {
			return fmt.Errorf("invalid -L spec: %q (expected [bind_addr:]port:host:hostport)", lf)
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
			return fmt.Errorf("invalid -R spec: %q (expected [bind_addr:]port:host:hostport)", rf)
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
		return fmt.Errorf("proxy service not available")
	}

	if c.NArg() < 1 {
		return cli.ShowSubcommandHelp(c)
	}

	id := c.Args()[0]
	return a.proxySvc.StopTunnel(id)
}

func (a *App) tunnelListAction(c *cli.Context) error {
	if a.proxySvc == nil {
		return fmt.Errorf("proxy service not available")
	}

	tunnels := a.proxySvc.ListTunnels()
	if len(tunnels) == 0 {
		fmt.Println("No active tunnels")
		return nil
	}

	fmt.Printf("%-10s  %-6s  %-30s  %-30s\n", "ID", "TYPE", "LOCAL", "REMOTE")
	fmt.Println(strings.Repeat("-", 80))
	for _, t := range tunnels {
		fmt.Printf("%-10s  %-6s  %-30s  %-30s\n", t.ID(), t.Type(), t.LocalAddr(), t.RemoteAddr())
	}
	fmt.Printf("\nTotal: %d tunnel(s)\n", len(tunnels))
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
