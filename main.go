package main

import (
	"assh/src"
	"assh/src/assh"
	"fmt"
	"github.com/keesely/kiris"
	//"github.com/keesely/kiris/hash"
	"github.com/urfave/cli"
	"os"
	"reflect"
)

//main
func main() {
	// 加密测试
	//str := "这是一段加密文本"
	//key := hash.Md5("keesely.net")
	//fmt.Println("Key: ", key, " LEN: ", len(key))
	//encrypt := kiris.AESEncrypt(str, key, "cbc")
	//fmt.Printf("加密后的文本: %s \n", encrypt)
	//decrypt := kiris.AESDecrypt(encrypt, key, "cbc")
	//fmt.Printf("解码: %s \n", decrypt)
	app := src.NewCli()
	app.Runtime.Commands = []cli.Command{
    app.ListServers(),
    app.Keygen(),
    app.SetPasswd(),
		{
			Name:  "add",
			Usage: "add the server",
			Action: func(c *cli.Context) error {
				cnf := assh.NewAssh()
				addServer(cnf)
				return nil
			},
		},
		{
			Name:    "remove",
			Aliases: []string{"rm"},
			Usage:   "reomve the server",
			Action: func(c *cli.Context) error {
				cnf := assh.NewAssh()
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
			Aliases: []string{"conn", "c", "login", "sign"},
			Usage:   "connection to login server",
			Action: func(c *cli.Context) error {
				cnf := assh.NewAssh()
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
				cnf := assh.NewAssh()
				name := c.Args().First()
				showServerDetail(name, cnf)
				return nil
			},
		},
	}

	app.Runtime.Run(os.Args)
}

func showServerDetail(name string, cnf *assh.Assh) {
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

func addServer(cnf *assh.Assh) error {
	keys := []string{"Name", "Host", "Port", "User", "Password", "PemKey"}
	var server = new(assh.Server)
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

func editServer(name string, cnf *assh.Assh) error {
	return nil
}
