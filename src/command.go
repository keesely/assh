// command.go kee > 2019/11/10

package src

import (
	"assh/src/assh"
	"assh/src/keygen"
	"fmt"
	"github.com/keesely/kiris"
	"github.com/urfave/cli"
	"reflect"
)

type App struct {
	Runtime *cli.App
	args    []string
}

var commands []string

func NewCli(args []string) *App {
	app := cli.NewApp()
	app.Name = "assh"
	app.Usage = "欢迎使用 Assh 工具"
	app.Version = "0.0.1"
	_app := &App{app, args}
	_app.command()
	return _app
}

func (app *App) command() {
	app.Runtime.Commands = []cli.Command{
		app.ListServers(),
		app.Keygen(),
		app.SetPasswd(),
		app.AddServer(),
		app.RemoveServer(),
		app.Connection(),
		app.PushFiles(),
		app.PullFiles(),
		app.ServerInfo(),
		app.MoveServer(),
		app.SetRemark(),
	}

	app.Runtime.Action = func(c *cli.Context) error {
		name := c.Args().First()
		cmd := c.Args().Get(1)
		ss := assh.NewAssh()
		if s := ss.GetServer(name); s != nil {
			loginServer(s, cmd)
			return nil
		}
		return fmt.Errorf("Login %s fail: not found the server config\n", name)
	}
}

func (app *App) Run() {
	app.Runtime.Run(app.args)
}

func cSet(name string) string {
	commands = append(commands, name)
	return name
}

func (app *App) Keygen() cli.Command {
	return cli.Command{
		Name:    cSet("keygen"),
		Aliases: []string{cSet("key")},
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
		Name:  cSet("sign"),
		Usage: "设定安全启动密码",
		Action: func(c *cli.Context) error {
			passwd := c.Args().First()
			assh.SetPasswd(passwd)
			fmt.Println("The password set success")
			return nil
		},
	}
}

func (app *App) ListServers() cli.Command {
	return cli.Command{
		Name:  cSet("ls"),
		Usage: "打印服务器列表信息",
		Action: func(c *cli.Context) error {
			fmt.Println("Server List")
			ss := assh.NewAssh()
			ss.ListServers()
			return nil
		},
	}
}

func (app *App) AddServer() cli.Command {
	host := cli.StringFlag{Name: "host", Value: "", Usage: "server host"}
	port := cli.IntFlag{Name: "P", Value: 22, Usage: "server port"}
	user := cli.StringFlag{Name: "u", Value: "root", Usage: "server login user name"}
	passwd := cli.StringFlag{Name: "p", Value: "", Usage: "server login password"}
	pubKey := cli.StringFlag{Name: "k", Value: "", Usage: "Automatic login public key"}
	remark := cli.StringFlag{Name: "remark", Value: "", Usage: "Server remark"}
	return cli.Command{
		Name:  cSet("add"),
		Usage: "add the server",
		Flags: []cli.Flag{host, port, user, passwd, pubKey, remark},
		Action: func(c *cli.Context) error {
			var s = assh.Server{}
			ss := assh.NewAssh()

			args := c.Args()
			if len(args) < 1 {
				addServer(&s, ss)
			} else {
				s.Name = args.First()
				s.Host = c.String("host")
				s.Port = c.Int("P")
				s.User = c.String("u")
				s.Password = c.String("p")
				s.PemKey = c.String("k")
				s.Remark = c.String("remark")
			}
			ss.AddServer(s.Name, s)
			return nil
		},
	}
}

func (app *App) RemoveServer() cli.Command {
	return cli.Command{
		Name:  cSet("rm"),
		Usage: "reomve the server",
		Action: func(c *cli.Context) error {
			name := c.Args().First()
			ss := assh.NewAssh()
			if name != "" {
				if s := ss.GetServer(name); s != nil {
					ss.DelServer(name)
					return nil
				} else {
					fmt.Printf("服务器(%s)不存在于服务器列表中.\n", name)
					return nil
				}
			} else {
				ss.ListServers()
				fmt.Printf("请输入要删除的服务器名称: ")
				fmt.Scanln(&name)
				if name != "" {
					ss.DelServer(name)
				}
				return nil
			}
		},
	}
}

func (app *App) Connection() cli.Command {
	host := cli.StringFlag{Name: "host", Value: "", Usage: "server host"}
	port := cli.IntFlag{Name: "P", Value: 22, Usage: "server port"}
	user := cli.StringFlag{Name: "u", Value: "root", Usage: "server login user name"}
	passwd := cli.StringFlag{Name: "p", Value: "", Usage: "server login password"}
	pubKey := cli.StringFlag{Name: "k", Value: "", Usage: "Automatic login public key"}
	remark := cli.StringFlag{Name: "remark", Value: "", Usage: "Server remark"}
	command := cli.StringFlag{Name: "c", Value: "", Usage: "run Command"}
	return cli.Command{
		Name:  cSet("login"),
		Usage: "connection to login server",
		Flags: []cli.Flag{host, port, user, passwd, pubKey, remark, command},
		Action: func(c *cli.Context) error {
			cmd := c.String("c")

			args := c.Args()

			if len(args) >= 1 {
				name := c.Args().First()
				ss := assh.NewAssh()
				if s := ss.GetServer(name); s != nil {
					loginServer(s, cmd)
					return nil
				}
				return fmt.Errorf("Login %s fail: not found the server config\n", name)
			} else {
				s := &assh.Server{}

				s.Host = c.String("host")
				s.Name = s.Host
				s.Port = c.Int("P")
				s.User = c.String("u")
				s.Password = c.String("p")
				s.PemKey = c.String("k")
				s.Remark = c.String("remark")

				loginServer(s, cmd)
			}
			return nil
		},
	}
}

func (app *App) PushFiles() cli.Command {
	return cli.Command{
		Name:  cSet("push"),
		Usage: "scp put file to remote server",
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			var (
				local  []string
				remote string
			)
			args := c.Args()
			name := args.First()
			if len(args) > 2 {
				local = args[1 : len(args)-1]
				remote = args[len(args)-1]
			} else {
				local = args[1:]
				remote = "~/"
			}

			if 0 >= len(local) {
				fmt.Printf("请选择需要上传的本地文件\n")
				return nil
			}
			ss := assh.NewAssh()
			s := ss.GetServer(name)
			if s != nil {
				err := s.ScpPushFiles(local, remote)
				if err != nil {
					fmt.Println(err)
				}
				return nil
			}
			return fmt.Errorf("Login %s fail: not found the server config\n", name)
			return nil
		},
	}
}

func (app *App) PullFiles() cli.Command {
	return cli.Command{
		Name:  cSet("pull"),
		Usage: "scp get file from remote server",
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			var (
				local  string
				remote []string
			)
			args := c.Args()
			name := args.First()
			if len(args) > 2 {
				remote = args[1 : len(args)-1]
				local = args[len(args)-1]
			} else {
				remote = args[1:]
				local = "./"
			}

			if 0 >= len(remote) {
				fmt.Printf("请选择需要获取的远程文件\n")
				return nil
			}
			ss := assh.NewAssh()
			s := ss.GetServer(name)
			if s != nil {
				err := s.ScpPullFiles(remote, local)
				if err != nil {
					fmt.Println(err)
				}
				return nil
			}
			return fmt.Errorf("Login %s fail: not found the server config\n", name)
			return nil
		},
	}
}

func (app *App) ServerInfo() cli.Command {
	return cli.Command{
		Name:  cSet("info"),
		Usage: "show the server detail info",
		Action: func(c *cli.Context) error {
			name := c.Args().First()
			if name == "" {
				fmt.Println("请输入您服务器名称")
				return nil
			}
			fmt.Println(kiris.StrPad("", "=", 100, 0))
			fmt.Println(kiris.StrPad(" Server Information Detail ", "+", 100, kiris.KIRIS_STR_PAD_BOTH))
			fmt.Println(kiris.StrPad("", "-", 100, 0))
			ss := assh.NewAssh()
			if s := ss.GetServer(name); s != nil {
				ss := reflect.ValueOf(s).Elem()
				for i, k := range []string{"Name", "Host", "Port", "User", "Password", "PemKey"} {
					fmt.Printf("%20s:   %v\n", "Server "+k, ss.Field(i))
				}
			} else {
				fmt.Printf("服务器(%s) 不存在\n", name)
			}
			fmt.Println(kiris.StrPad("", "=", 100, 0))
			return nil
		},
	}
}

func (app *App) SetRemark() cli.Command {
	return cli.Command{
		Name:  cSet("remark"),
		Usage: "设置服务器备注",
		Action: func(c *cli.Context) error {
			name := c.Args().First()
			remark := c.Args().Get(1)
			ss := assh.NewAssh()
			ss.SetRemark(name, remark)
			return nil
		},
	}
}

func (app *App) MoveServer() cli.Command {
	return cli.Command{
		Name:  cSet("mv"),
		Usage: "移动服务器分组",
		Action: func(c *cli.Context) error {
			name := c.Args().First()
			toGroup := c.Args().Get(1)
			ss := assh.NewAssh()
			ss.MoveServer(name, toGroup)
			return nil
		},
	}
}

func addServer(server *assh.Server, ss *assh.Assh) error {
	keys := []string{"Name", "Host", "Port", "User", "Password", "PemKey"}
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
			if s := ss.GetServer(server.Name); s != nil {
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
	return nil
}

func loginServer(s *assh.Server, cmd string) {
	if cmd != "" {
		s.Command(cmd)
	}
	s.Connection()

	if cout := s.CombinedOutput(); cout != "" {
		fmt.Printf("> Result (%s): \n%s\n", s.Name, cout)
	}
}
