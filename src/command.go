// command.go kee > 2019/11/10

package src

import (
	"github.com/urfave/cli"
)

type App struct {
	app *cli.App
}

func NewCli() *App {
	app := cli.NewApp()
	app.Name = "gossh"
	app.Usage = "欢迎使用 goSSH 工具"
	app.Version = "0.0.1"
	return &App{app}
}

func (app *App) list() cli.Command {
	return cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "打印服务器列表信息",
		Action: func(c *cli.Context) error {
			return nil
		},
	}
}
