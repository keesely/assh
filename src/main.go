package main

import (
	"fmt"
	"github.com/keesely/kiris"
	"github.com/urfave/cli"
	"gossh/src/config"
	"gossh/src/gossh"
	"os"
	"reflect"
)

func main() {
	cnf := config.NewConfig()
	app := cli.NewApp()
	app.Name = "gossh"
	app.Usage = "欢迎使用 goSSH 服务"
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
		{
			Name:    "list",
			Aliases: []string{"ls"},
			Usage:   "show the server list",
			Action: func(c *cli.Context) error {
				fmt.Println("Server List")
				cnf.ListServers()
				return nil
			},
		},
		{
			Name:  "add",
			Usage: "add the server",
			Action: func(c *cli.Context) error {
				addServer(cnf)
				return nil
			},
		},
		{
			Name:    "remove",
			Aliases: []string{"rm"},
			Usage:   "reomve the server",
			Action: func(c *cli.Context) error {
				name := c.Args().First()
				if name != "" {
					if s := cnf.GetServer(name); s != nil {
						cnf.DelServer(name)
						return nil
					} else {
						fmt.Printf("服务器(%s)不存在于服务器列表中.\n", name)
						return nil
					}
				} else {
					cnf.ListServers()
					fmt.Printf("请输入要删除的服务器名称: ")
					fmt.Scanln(&name)
					if name != "" {
						cnf.DelServer(name)
					}
					return nil
				}
			},
		},
		{
			Name:    "connect",
			Aliases: []string{"conn", "c"},
			Usage:   "connection to login server",
			Action: func(c *cli.Context) error {
				name := c.Args().First()
				if s := cnf.GetServer(name); s != nil {
					s.Connection()
					return nil
				}
				return fmt.Errorf("Login %s fail: not found the server config\n", name)
			},
		},
		{
			Name:    "info",
			Aliases: []string{"i"},
			Usage:   "get the server detail info",
			Action: func(c *cli.Context) error {
				name := c.Args().First()
				showServerDetail(name, cnf)
				return nil
			},
		},
	}

	app.Run(os.Args)
}

func showServerDetail(name string, cnf *config.Config) {
	if name == "" {
		fmt.Println("请输入您服务器名称")
		return
	}
	fmt.Println(kiris.StrPad("", "=", 100, 0))
	fmt.Println(kiris.StrPad(" Server Information Detail ", "+", 100, kiris.KIRIS_STR_PAD_BOTH))
	fmt.Println(kiris.StrPad("", "-", 100, 0))
	if s := cnf.GetServer(name); s != nil {
		ss := reflect.ValueOf(s).Elem()
		for i, k := range []string{"Name", "Host", "Port", "User", "Password", "PemKey"} {
			fmt.Printf("%20s:   %v\n", "Server "+k, ss.Field(i))

		}
	} else {
		fmt.Printf("服务器(%s) 不存在\n", name)
	}
	fmt.Println(kiris.StrPad("", "=", 100, 0))
}

func addServer(cnf *config.Config) error {
	keys := []string{"Name", "Host", "Port", "User", "Password", "PemKey"}
	var server = new(gossh.Server)
	fmt.Println("请按照提示填入服务器信息(标记* 为必要填写项目): ")
	for i, key := range keys {
		var opt string
		if "Password" != key && "PemKey" != key {
			opt = "*"
		}
		fmt.Printf("%d. Please input [%s%s] > ", i+1, opt, key)
		Errorf := func(msg string) error {
			fmt.Printf("\033[6;31;40m  %s\n", msg)
			return nil
		}
		ReqErrorf := func(key string) error {
			fmt.Printf("\033[6;31;40m  !! 参数 [%s] 为必要填写项目.\n", key)
			return nil
		}
		switch key {
		case "Name":
			fmt.Scanln(&server.Name)
			if server.Name == "" {
				return ReqErrorf(key)
			}
			if s := cnf.GetServer(server.Name); s != nil {
				return Errorf("服务器项(" + server.Name + ")已存在，请确认.")
			}
		case "Host":
			fmt.Scanln(&server.Host)
			if server.Host == "" {
				return ReqErrorf(key)
			}
		case "Port":
			fmt.Scanln(&server.Port)
			if server.Port == 0 {
				return ReqErrorf(key)
			}
		case "User":
			fmt.Scanln(&server.User)
			if server.User == "" {
				return ReqErrorf(key)
			}
		case "Password":
			fmt.Scanln(&server.Password)
		case "PemKey":
			fmt.Scanln(&server.Password)
		}
		//fmt.Printf("\n")
	}
	cnf.AddServer(server.Name, server)
	return nil
}

func editServer(name string, cnf *config.Config) error {
	return nil
}
