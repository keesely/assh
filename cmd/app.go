package cmd

import (
	"fmt"

	"github.com/urfave/cli"
)

type App struct {
	cli     *cli.App
	version string
	build   string
}

func NewApp(version, build string) *App {
	app := cli.NewApp()
	app.Name = "ASSH - An SSH Client"
	app.Usage = "An SSH Client"
	app.Version = version
	app.EnableBashCompletion = true

	a := &App{
		cli:     app,
		version: version,
		build:   build,
	}
	a.setupGlobalFlags()
	a.registerCommands()
	return a
}

func (a *App) Run(args []string) {
	a.cli.Run(args)
}

func (a *App) setupGlobalFlags() {
	a.cli.Flags = []cli.Flag{
		cli.StringFlag{Name: "log", Usage: "log file path"},
		cli.StringFlag{Name: "llv", Usage: "log level"},
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
}

func (a *App) versionAction(c *cli.Context) error {
	fmt.Println(a.version)
	return nil
}

func lookupShortFlag(c *cli.Context, flag string) interface{} {
	for i, argv := range c.Args() {
		if argv == "-"+flag {
			return c.Args().Get(i + 1)
		}
	}
	return nil
}

func init() {
	cli.AppHelpTemplate = fmt.Sprintf(`%s

ENVIRONMENT:
   ASSH_CONFIG_DIR   config directory (default: ~/.assh)
   ASSH_LOG_FILE    log file path
   ASSH_LOG_LEVEL   log level (OFF/DEBUG/INFO/WARN/ERROR/FATAL)

`, cli.AppHelpTemplate)
}
