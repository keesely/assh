package cmd

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"assh/asshc/port"

	"github.com/urfave/cli"
)

func (a *App) registerServiceCommands() {
	// service 子命令注册为顶级命令
	a.cli.Commands = append(a.cli.Commands, cli.Command{
		Name:  "service",
		Usage: "Manage proxy and tunnel services",
		Subcommands: []cli.Command{
			{
				Name:   "list",
				Usage:  "List registered services",
				Action: a.serviceListAction,
				Flags: []cli.Flag{
					cli.StringFlag{Name: "type", Value: "", Usage: "filter by type: proxy|tunnel"},
					cli.StringFlag{Name: "status", Value: "", Usage: "filter by status: running|stopped|failed"},
					cli.StringFlag{Name: "server", Value: "", Usage: "filter by server name"},
				},
			},
			{
				Name:      "start",
				Usage:     "Start a service",
				ArgsUsage: "<id>",
				Action:    a.serviceStartAction,
			},
			{
				Name:      "stop",
				Usage:     "Stop a service",
				ArgsUsage: "<id>",
				Action:    a.serviceStopAction,
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "all", Usage: "stop all running services"},
					cli.StringFlag{Name: "type", Value: "", Usage: "stop all services of type: proxy|tunnel"},
				},
			},
			{
				Name:      "restart",
				Usage:     "Restart a service",
				ArgsUsage: "<id>",
				Action:    a.serviceRestartAction,
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "all", Usage: "restart all running services"},
					cli.StringFlag{Name: "type", Value: "", Usage: "restart all services of type: proxy|tunnel"},
				},
			},
			{
				Name:      "rm",
				Usage:     "Remove a service registration",
				ArgsUsage: "<id>",
				Action:    a.serviceRmAction,
			},
		},
	})
}

func (a *App) serviceListAction(c *cli.Context) error {
	if a.serviceMgr == nil {
		return fmt.Errorf("service manager not available")
	}

	// 构建过滤条件
	filter := port.ServiceFilter{}
	if typeStr := c.String("type"); typeStr != "" {
		filter.Type = port.ServiceType(typeStr)
	}
	if statusStr := c.String("status"); statusStr != "" {
		filter.Status = port.ServiceStatus(statusStr)
	}
	filter.Server = c.String("server")

	// 查询服务列表
	services, err := a.serviceMgr.List(filter)
	if err != nil {
		return fmt.Errorf("list services: %w", err)
	}

	// 输出表格
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tNAME\tTYPE\tSERVER\tSTATUS\tREMARK\n")
	for _, svc := range services {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			svc.ID, svc.Name, svc.Type, svc.ServerName, svc.Status, svc.Remark)
	}
	w.Flush()

	return nil
}

func (a *App) serviceStartAction(c *cli.Context) error {
	if a.serviceMgr == nil {
		return fmt.Errorf("service manager not available")
	}

	if !c.Args().Present() {
		return fmt.Errorf("service ID required")
	}

	id, err := strconv.ParseInt(c.Args().First(), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid service ID: %w", err)
	}

	// 获取服务实例信息
	instance, err := a.serviceMgr.Status(id)
	if err != nil {
		return fmt.Errorf("get service status: %w", err)
	}

	// 启动服务
	if err := a.serviceMgr.Start(&port.ServiceInstance{ID: id}); err != nil {
		return fmt.Errorf("start service: %w", err)
	}

	fmt.Printf("service %d started (status: %s)\n", id, instance)
	return nil
}

func (a *App) serviceStopAction(c *cli.Context) error {
	if a.serviceMgr == nil {
		return fmt.Errorf("service manager not available")
	}

	// 处理 --all 参数
	if c.Bool("all") {
		return a.stopAllServices(c.String("type"))
	}

	if !c.Args().Present() {
		return fmt.Errorf("service ID required or use --all")
	}

	id, err := strconv.ParseInt(c.Args().First(), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid service ID: %w", err)
	}

	// 停止服务
	if err := a.serviceMgr.Stop(id); err != nil {
		return fmt.Errorf("stop service: %w", err)
	}

	fmt.Printf("service %d stopped\n", id)
	return nil
}

func (a *App) stopAllServices(typeFilter string) error {
	filter := port.ServiceFilter{}
	if typeFilter != "" {
		filter.Type = port.ServiceType(typeFilter)
	}
	filter.Status = port.ServiceStatusRunning

	services, err := a.serviceMgr.List(filter)
	if err != nil {
		return fmt.Errorf("list services: %w", err)
	}

	stopped := 0
	for _, svc := range services {
		if err := a.serviceMgr.Stop(svc.ID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: stop service %d: %v\n", svc.ID, err)
			continue
		}
		stopped++
	}

	fmt.Printf("stopped %d services\n", stopped)
	return nil
}

func (a *App) serviceRestartAction(c *cli.Context) error {
	if a.serviceMgr == nil {
		return fmt.Errorf("service manager not available")
	}

	// 处理 --all 参数
	if c.Bool("all") {
		return a.restartAllServices(c.String("type"))
	}

	if !c.Args().Present() {
		return fmt.Errorf("service ID required or use --all")
	}

	id, err := strconv.ParseInt(c.Args().First(), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid service ID: %w", err)
	}

	// 重启服务
	if err := a.serviceMgr.Restart(id); err != nil {
		return fmt.Errorf("restart service: %w", err)
	}

	fmt.Printf("service %d restarted\n", id)
	return nil
}

func (a *App) restartAllServices(typeFilter string) error {
	filter := port.ServiceFilter{}
	if typeFilter != "" {
		filter.Type = port.ServiceType(typeFilter)
	}
	filter.Status = port.ServiceStatusRunning

	services, err := a.serviceMgr.List(filter)
	if err != nil {
		return fmt.Errorf("list services: %w", err)
	}

	restarted := 0
	for _, svc := range services {
		if err := a.serviceMgr.Restart(svc.ID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: restart service %d: %v\n", svc.ID, err)
			continue
		}
		restarted++
	}

	fmt.Printf("restarted %d services\n", restarted)
	return nil
}

func (a *App) serviceRmAction(c *cli.Context) error {
	if !c.Args().Present() {
		return fmt.Errorf("service ID required")
	}

	id, err := strconv.ParseInt(c.Args().First(), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid service ID: %w", err)
	}

	// 尝试从代理服务删除
	if a.proxyRepo != nil {
		if err := a.proxyRepo.Delete(id); err == nil {
			fmt.Printf("proxy service %d removed\n", id)
			return nil
		}
	}

	// 尝试从隧道服务删除
	if a.tunnelRepo != nil {
		if err := a.tunnelRepo.Delete(id); err == nil {
			fmt.Printf("tunnel service %d removed\n", id)
			return nil
		}
	}

	return fmt.Errorf("service %d not found", id)
}
