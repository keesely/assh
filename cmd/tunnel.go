package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"assh/asshc/domain"
	"assh/asshc/service"
	"assh/config"

	"github.com/urfave/cli"
)

func (a *App) registerTunnelCommands() {
	// 注意：tunnel 命令不能有 Subcommands，否则 urfave/cli v1 不会解析主命令的 flags。
	// 所有子命令注册为顶级命令：tunnel-stop, tunnel-list, tunnel-start, tunnel-rm。
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:      "tunnel",
		Usage:     "Manage SSH port forwarding tunnels",
		ArgsUsage: "<server> [-L/-R spec]",
		Description: `Start SSH port forwarding tunnels.

Examples:
  assh tunnel myserver -L 8080:localhost:80        # local forward
  assh tunnel myserver -R 8080:localhost:80        # remote forward
  assh tunnel myserver -L 80:web:80 -L 443:web:443  # multiple forwards
  assh tunnel myserver -L 8080:localhost:80 --daemon  # background
  assh tunnel myserver -L 8080:localhost:80 --save my-tunnel  # save and start
  assh tunnel-list                                 # list saved tunnels
  assh tunnel-start my-tunnel                      # start saved tunnel
  assh tunnel-stop my-tunnel                       # stop tunnel
`,
		Flags: []cli.Flag{
			cli.StringSliceFlag{Name: "L", Usage: "local forward spec [bind_addr:]port:host:hostport"},
			cli.StringSliceFlag{Name: "R", Usage: "remote forward spec [bind_addr:]port:host:hostport"},
			cli.StringFlag{Name: "listen", Value: "tcp", Usage: "listen type: tcp|http"},
			cli.BoolFlag{Name: "daemon, d", Usage: "run in background as daemon"},
			cli.StringFlag{Name: "auto-reconnect", Usage: "auto reconnect [retries/]interval"},
			cli.StringFlag{Name: "J, jump", Usage: "jump host chain (comma-separated): name,user@host[:port]"},
			cli.StringFlag{Name: "save", Value: "", Usage: "save tunnel configuration with name"},
			cli.StringFlag{Name: "pid-file", Value: "", Usage: "PID file path for daemon mode (default: ~/.assh/v2/tunnel.pid)"},
		},
		Action: a.tunnelAction,
	})

	// tunnel 子命令注册为顶级命令（避免 urfave/cli v1 的 Subcommands flag 解析 bug）
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:      "tunnel-stop",
		Usage:     "Stop a running tunnel",
		ArgsUsage: "<id>",
		Action:    a.tunnelStopAction,
	})
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:   "tunnel-list",
		Usage:  "List saved tunnel configurations",
		Action: a.tunnelListAction,
	})
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:      "tunnel-start",
		Usage:     "Start a saved tunnel configuration",
		ArgsUsage: "<name|id>",
		Action:    a.tunnelStartAction,
	})
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:      "tunnel-rm",
		Usage:     "Remove a saved tunnel configuration",
		ArgsUsage: "<name|id>",
		Action:    a.tunnelRmAction,
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
	saveName := c.String("save")

	localForwards := c.StringSlice("L")
	remoteForwards := c.StringSlice("R")

	if len(localForwards) == 0 && len(remoteForwards) == 0 {
		return fmt.Errorf("at least one -L or -R flag is required")
	}

	var specs []service.ForwardSpec
	var forwards []domain.TunnelForward

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
		forwards = append(forwards, domain.TunnelForward{
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
			ListenType: listenType,
		})
		forwards = append(forwards, domain.TunnelForward{
			LocalAddr:  localAddr,
			RemoteAddr: remoteAddr,
			Reverse:    true,
		})
	}

	// 如果指定了 --save，保存配置
	if saveName != "" && a.tunnelRepo != nil {
		svc := &domain.TunnelService{
			Name:          saveName,
			GroupName:     "",
			ServerName:    serverName,
			JumpChain:     jumpChain,
			Forwards:      forwards,
			Daemon:        c.Bool("daemon"),
			AutoReconnect: c.String("auto-reconnect"),
			Enabled:       true,
		}

		if err := a.tunnelRepo.Create(svc); err != nil {
			return fmt.Errorf("save tunnel failed: %w", err)
		}
		fmt.Printf("Tunnel configuration saved as %q (id=%d)\n", saveName, svc.ID)
	}

	opts := service.ProxyOptions{
		Daemon:        c.Bool("daemon"),
		AutoReconnect: c.String("auto-reconnect"),
		ListenType:    listenType,
		PidPath:       c.String("pid-file"),
	}

	// 默认 PID 文件路径
	if opts.PidPath == "" && opts.Daemon {
		opts.PidPath = config.PidPath
	}

	if jumpChain != "" {
		return a.proxySvc.StartTunnelWithChain(serverName, jumpChain, specs, opts)
	}

	return a.proxySvc.StartTunnel(serverName, specs, opts)
}

// resolveTunnelService 根据名字或 ID 解析隧道服务
func (a *App) resolveTunnelService(nameOrID string) (*domain.TunnelService, error) {
	if a.tunnelRepo == nil {
		return nil, fmt.Errorf("tunnel repository not available")
	}

	// 尝试按 ID 解析
	if id, err := strconv.ParseInt(nameOrID, 10, 64); err == nil {
		return a.tunnelRepo.GetByID(id)
	}

	// 按名字解析
	return a.tunnelRepo.GetByName("", nameOrID)
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
	if a.tunnelRepo == nil {
		return fmt.Errorf("tunnel repository not available")
	}

	services, err := a.tunnelRepo.List("")
	if err != nil {
		return fmt.Errorf("list tunnels failed: %w", err)
	}

	if len(services) == 0 {
		fmt.Println("No saved tunnel configurations")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tNAME\tSERVER\tFORWARDS\tSTATUS\n")
	for _, svc := range services {
		status := "enabled"
		if !svc.Enabled {
			status = "disabled"
		}
		forwards := formatForwards(svc.Forwards)
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", svc.ID, svc.Name, svc.ServerName, forwards, status)
	}
	w.Flush()
	return nil
}

func formatForwards(forwards []domain.TunnelForward) string {
	if len(forwards) == 0 {
		return "-"
	}
	var parts []string
	for _, f := range forwards {
		if f.Reverse {
			parts = append(parts, fmt.Sprintf("-R %s:%s", f.LocalAddr, f.RemoteAddr))
		} else {
			parts = append(parts, fmt.Sprintf("-L %s:%s", f.LocalAddr, f.RemoteAddr))
		}
	}
	return strings.Join(parts, ", ")
}

func (a *App) tunnelStartAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return cli.ShowSubcommandHelp(c)
	}

	svc, err := a.resolveTunnelService(c.Args()[0])
	if err != nil {
		return fmt.Errorf("tunnel not found: %w", err)
	}

	if a.proxySvc == nil {
		return fmt.Errorf("proxy service not available")
	}

	var specs []service.ForwardSpec
	for _, f := range svc.Forwards {
		specs = append(specs, service.ForwardSpec{
			LocalAddr:  f.LocalAddr,
			RemoteAddr: f.RemoteAddr,
			Reverse:    f.Reverse,
		})
	}

	opts := service.ProxyOptions{
		Daemon:        svc.Daemon,
		AutoReconnect: svc.AutoReconnect,
	}

	// daemon 模式默认 PID 文件路径
	if opts.Daemon {
		opts.PidPath = config.PidPath
	}

	// 记录使用
	if a.tunnelRepo != nil {
		_ = a.tunnelRepo.RecordUsage(svc.ID)
	}

	if svc.JumpChain != "" {
		return a.proxySvc.StartTunnelWithChain(svc.ServerName, svc.JumpChain, specs, opts)
	}

	return a.proxySvc.StartTunnel(svc.ServerName, specs, opts)
}

func (a *App) tunnelRmAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return cli.ShowSubcommandHelp(c)
	}

	svc, err := a.resolveTunnelService(c.Args()[0])
	if err != nil {
		return fmt.Errorf("tunnel not found: %w", err)
	}

	if err := a.tunnelRepo.Delete(svc.ID); err != nil {
		return fmt.Errorf("delete tunnel failed: %w", err)
	}

	fmt.Printf("Tunnel %q (id=%d) removed\n", svc.Name, svc.ID)
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
