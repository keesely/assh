package cmd

import (
	"fmt"
	"os"

	"assh/asshc/service"
	"assh/config"
	"assh/log"

	"github.com/urfave/cli"
)

type App struct {
	cli        *cli.App
	version    string
	build      string
	connectSvc *service.ConnectService
	serverSvc  *service.ServerService
}

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
	a.registerCommands()
	return a
}

func (a *App) Run(args []string) error {
	return a.cli.Run(args)
}

func (a *App) setupGlobalFlags() {
	a.cli.Flags = []cli.Flag{
		cli.BoolFlag{Name: "v, verbose", Usage: "verbose output"},
		cli.BoolFlag{Name: "q, quiet", Usage: "quiet mode"},
		cli.StringFlag{Name: "F, config", Usage: "config file path (default: ~/.assh/v2/assh.yml)"},
		cli.BoolFlag{Name: "V, version", Usage: "print version information"},
	}
}

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

func (a *App) versionAction(c *cli.Context) error {
	fmt.Println(a.version)
	return nil
}

func firstNonEmpty(strs ...string) string {
	for _, s := range strs {
		if s != "" {
			return s
		}
	}
	return ""
}

func init() {
	cli.AppHelpTemplate = fmt.Sprintf(`%s

ENVIRONMENT:
   ASSH_CONFIG_DIR   config directory (default: ~/.assh/v2)

`, cli.AppHelpTemplate)

	_ = config.EnsureDir(config.DataPath)
	_ = os.MkdirAll("/tmp", 0755)
}
