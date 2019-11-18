// command.go kee > 2019/11/10

package src

import (
	"assh/src/assh"
	"assh/src/keygen"
	"fmt"
	"github.com/urfave/cli"
)

type App struct {
	app *cli.App
}

func NewCli() *App {
	app := cli.NewApp()
	app.Name = "assh"
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

func (app *App) Keygen() cli.Command {
	return cli.Command{
		Name:    "keygen",
		Aliases: []string{"key"},
		Usage:   "生成ssh公钥",
		Action: func(c *cli.Context) error {
			key, _ := keygen.NewRsa(2048)
			public, private, _ := key.GenSSHKey("")
			fmt.Println(public, "\n", private)
			return nil
		},
	}
}

func (app *App) SetPasswd() cli.Command {
	return cli.Command{
		Name:  "account",
		Usage: "设定安全启动密码",
		Action: func(c *cli.Context) error {
			passwd := c.Args().First()
			assh.SetPasswd(passwd)
			fmt.Println("The password set success")
			return nil
		},
	}
}
