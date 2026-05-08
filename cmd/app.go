package cmd

import (
	"fmt"
	"os"

	"assh/config"
	"assh/log"

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
	app.Before = beforeAction

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
		cli.StringFlag{Name: "llv", Usage: "log level (OFF/DEBUG/INFO/WARN/ERROR/FATAL)"},
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

func beforeAction(c *cli.Context) error {
	if logPath := c.String("log"); logPath != "" {
		log.LogPath = logPath
		log.SetInit()
	}

	if logLevel := c.String("llv"); logLevel != "" {
		log.LogLevel = log.GetLogLevel(logLevel)
		log.SetInit()
	}

	return nil
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
   ASSH_CONFIG_DIR   config directory (default: ~/.assh/v2)
   ASSH_LOG_FILE    log file path
   ASSH_LOG_LEVEL   log level (OFF/DEBUG/INFO/WARN/ERROR/FATAL)

`, cli.AppHelpTemplate)

	_ = config.EnsureDir(config.DataPath)
	_ = os.MkdirAll("/tmp", 0755)
}