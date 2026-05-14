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
		ArgsUsage: "start/stop/list",
		Subcommands: []cli.Command{
			{
				Name:      "start",
				Usage:     "Start a port forwarding tunnel",
				ArgsUsage: "<server> [-L [bind:]port:host:hostport] [-R [bind:]port:host:hostport]",
				Flags: []cli.Flag{
					cli.StringSliceFlag{Name: "L", Usage: "local forward spec [bind_addr:]port:host:hostport"},
					cli.StringSliceFlag{Name: "R", Usage: "remote forward spec [bind_addr:]port:host:hostport"},
					cli.BoolFlag{Name: "daemon, d", Usage: "run in background as daemon"},
					cli.StringFlag{Name: "auto-reconnect", Usage: "auto reconnect [retries/]interval"},
				},
				Action: a.tunnelStartAction,
			},
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

func (a *App) tunnelStartAction(c *cli.Context) error {
	if a.proxySvc == nil {
		return fmt.Errorf("proxy service not available")
	}

	if c.NArg() < 1 {
		return cli.ShowSubcommandHelp(c)
	}

	serverName := c.Args()[0]

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
		})
	}

	opts := service.ProxyOptions{
		Daemon:        c.Bool("daemon"),
		AutoReconnect: c.String("auto-reconnect"),
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
		return fmt.Sprintf("127.0.0.1:%s", parts[0]), fmt.Sprintf("%s:%s", parts[1], parts[2])
	case 4:
		return fmt.Sprintf("%s:%s", parts[0], parts[1]), fmt.Sprintf("%s:%s", parts[2], parts[3])
	default:
		return "", ""
	}
}
