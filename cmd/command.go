// command.go kee > 2019/12/02

package cmd

import (
	assh "assh/asshc"
	"assh/cmd/qiniu"
	"assh/log"
	"fmt"
	"os"

	"github.com/keesely/kiris"
	"github.com/keesely/kiris/hash"
	"github.com/urfave/cli"

	//"reflect"
	"strings"
	"time"
)

type App struct {
	app *cli.App
}

type server struct {
	Name     string            `yaml:"name"`
	Host     string            `yaml:"host"`
	Port     int               `yaml:"port"`
	User     string            `yaml:"user"`
	Password string            `yaml:"password"`
	Remark   string            `yaml:"remark"`
	PemKey   map[string]string `yaml:"pemkey"`
}

var (
	version = "v1.0.5-20240724.fix"

	commonFlags = []cli.Flag{
		cli.StringFlag{Name: "H", Value: "", Usage: "server host"},
		cli.IntFlag{Name: "p", Value: 0, Usage: "bind address port"},
		cli.StringFlag{Name: "l", Value: "", Usage: "login name"},
		cli.StringFlag{Name: "P", Value: "", Usage: "login password"},
		cli.StringFlag{Name: "k", Value: "", Usage: "automatic login public key"},
	}

	commands = []cli.Command{
		cli.Command{
			Name:   "version",
			Action: Version,
		},
		cli.Command{
			Name:   "account",
			Usage:  "Set the data crypto key (password)",
			Action: Account,
		},
		cli.Command{
			Name:   "ls",
			Usage:  "Print all servers list",
			Action: ListServer,
		},
		cli.Command{
			Name:   "search",
			Usage:  "(New) Search the server name",
			Action: SearchServer,
		},
		cli.Command{
			Name:   "info",
			Usage:  "Print the server information",
			Action: InfoServer,
		},
		cli.Command{
			Name:  "set",
			Usage: "Add/Modify the server",
			Flags: append(commonFlags,
				cli.StringFlag{Name: "R", Value: "", Usage: "Server remark"},
				cli.BoolFlag{Name: "f", Usage: "force set, don't tip"},
			),
			Action: SetServer,
		},
		cli.Command{
			Name:   "mv",
			Usage:  "Move/Rename Server Name",
			Action: MoveServer,
		},
		cli.Command{
			Name:  "rm",
			Usage: "Remove Server",
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "f", Usage: "force delete, don't tip"},
				cli.BoolFlag{Name: "g", Usage: "delete group"},
			},
			Action: RemoveServer,
		},
		cli.Command{
			Name:   "login",
			Usage:  "login server or exec remote server command",
			Flags:  commonFlags,
			Action: Connection,
		},
		cli.Command{
			Name:  "bc",
			Usage: "batch remote command execution",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "f", Usage: "the Bash script file"},
			},
			Action: BatchCommand,
		},
		cli.Command{
			Name:  "push",
			Usage: "sftp push file to remote server",
			Flags: []cli.Flag{
				cli.IntFlag{Name: "b", Value: 1048576, Usage: "write up to BYTES bytes at a time (default: 1048576 = 1M)"},
			},
			Action: ScpPush,
		},
		cli.Command{
			Name:   "pull",
			Usage:  "sftp pull file from remote server",
			Action: ScpPull,
		},
		cli.Command{
			Name:  "keygen",
			Usage: "ssh keygen",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "c", Usage: "ssh keygen comment"},
				cli.StringFlag{Name: "f", Usage: "save file name", Value: "./id_rsa"},
			},
			Action: Keygen,
		},
		cli.Command{
			Name:  "sync",
			Usage: "synchronized data to the cloud storage",
			Subcommands: []cli.Command{
				cli.Command{
					Name:   "account",
					Usage:  "cloud storage account",
					Action: SyncAccount,
				},
				cli.Command{
					Name:  "push",
					Usage: "push local data to storage",
					Flags: []cli.Flag{
						cli.BoolFlag{Name: "d", Usage: "whether the backup in date dirctory"},
					},
					Action: SyncUp,
				},
				cli.Command{
					Name:   "pull",
					Usage:  "pull storage data",
					Action: SyncDown,
				},
			},
		},
		cli.Command{
			Name:   "export",
			Action: ExportData,
		},
		cli.Command{
			Name: "import",
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "f", Usage: "force import, don't tip"},
			},
			Action: ImportData,
		},

		cli.Command{
			Name: "upgrade",
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "f", Usage: "force upgrade, don't tips"},
			},
			Action: Upgrade,
		},
		cli.Command{
			Name:   "ping",
			Action: PingServers,
		},
		cli.Command{
			Name:  "proxy",
			Usage: "assh proxy -d local port -i local hostname",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "d", Usage: "local port"},
				cli.StringFlag{Name: "i", Usage: "local hostname", Value: ""},
			},
			Action: Proxy,
		},
		cli.Command{
			Name:  "hostproxy",
			Usage: "assh prots -h remote host -p remote port -d local port -i local host [hostname]",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "H", Usage: "remote host"},
				cli.StringFlag{Name: "P", Usage: "remote port"},
				cli.StringFlag{Name: "d", Usage: "local port"},
				cli.StringFlag{Name: "i", Usage: "local host", Value: ""},
			},
			Action: ProxyHost,
		},
		cli.Command{
			Name:  "localproxy",
			Usage: "assh prots -h remote host -p remote port -d local port -i local host [hostname]",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "H", Usage: "remote host"},
				cli.StringFlag{Name: "P", Usage: "remote port"},
				cli.StringFlag{Name: "d", Usage: "local port"},
				cli.StringFlag{Name: "i", Usage: "local host", Value: ""},
			},
			Action: LocalProxy,
		},
	}
)

func NewCli() *App {
	app := cli.NewApp()
	app.Name = "Assh - An SSH Client"
	app.Usage = ""
	app.Version = version
	app.EnableBashCompletion = true

	return &App{app}
}

func (cmd *App) GetCliApp() *cli.App {
	return cmd.app
}

func (cmd *App) Run() {
	cmd.cmdAction()
	cmd.app.Commands = commands
	cmd.app.Run(os.Args)
}

// 基础命令
func (cmd *App) cmdAction() {
	cmd.app.Flags = append(commonFlags,
		cli.StringFlag{Name: "c", Value: "", Usage: "run command"},
		cli.StringFlag{Name: "log", Usage: "log file path"},
		cli.StringFlag{Name: "llv", Usage: "log level"},
	)

	cmd.app.Action = func(c *cli.Context) (err error) {
		if logPath := c.String("log"); logPath != "" {
			assh.SetLogPath(logPath)
		}
		if logLevel := c.String("llv"); logLevel != "" {
			assh.SetLogLevel(logLevel)
		}
		shortArgs := []string{"H", "p", "l", "P", "k"}
		var vf = false
		for _, f := range shortArgs {
			if x := lookupShortFlag(c, f); x != nil {
				c.Set(f, x.(string))
				vf = true
			} else if c.IsSet(f) {
				vf = true
			}
		}

		if vf || 0 < len(c.Args()) {
			return Connection(c)
		}
		return

	}
}

// 权限设置
func Account(c *cli.Context) (err error) {
	var (
		cPasswd string
		nPasswd string
		aPasswd string
	)
	nPasswd = c.Args().First()
	if assh.HasPasswd() {
		if passwd := assh.GetPasswd(); "" != passwd {
			fmt.Print("Current Password: ")
			fmt.Scanln(&cPasswd)
			if passwd != hash.Md5(cPasswd) {
				log.Error("passwd: Authentication failed")
				return
			}
		}
	}
	if nPasswd == "" {
		fmt.Print("New Password: ")
		fmt.Scanln(&nPasswd)
	}
	fmt.Print("Retrype Passwod: ")
	fmt.Scanln(&aPasswd)
	if hash.Md5(nPasswd) != hash.Md5(aPasswd) {
		log.Error("passwd: Inconsistent, please try again.")
		return
	}
	assh.SetPasswd(nPasswd, cPasswd)
	log.Info("passwd: Set new password")
	fmt.Println("success.")
	return
}

// 同步帐号设置
func SyncAccount(c *cli.Context) (err error) {
	args := c.Args()
	if len(args) < 3 {
		fmt.Println("Assh sync: account <accessKey> <secretKey> <bucket> to set sync service account")
		return nil
	}
	accessKey, secretKey, bucket := args.Get(0), args.Get(1), args.Get(2)
	assh.SetQiniuAccessKey(accessKey, secretKey, bucket)
	return
}

// 同步上传
func SyncUp(c *cli.Context) (err error) {
	src := kiris.RealPath("~/.assh/assh.zip")
	err = Zip(assh.GetDbPath(), src)
	if err != nil {
		log.Fatal(err)
		return
	}
	ident := c.Args().First()
	if ident == "" {
		ident = "backup"
	}

	var qN *qiniu.Qiniu
	if qN, err = getQiniu(); err != nil {
		log.Fatal(err)
		return
	}

	err = qN.Upload(src, "assh/"+ident+".zip")
	if err != nil {
		log.Fatal(err)
	}

	// 是否按时间保存
	if c.IsSet("d") {
		date := time.Now().Format("2006-01-02/1504")
		qN.Upload(src, "assh/"+date+"/"+ident+".zip")
	}

	fmt.Println("sync upload success.")
	return
}

// 同步下载
func SyncDown(c *cli.Context) (err error) {
	ident := c.Args().First()
	if ident == "" {
		ident = "backup"
	}
	dst := assh.GetDbPath() + "/" + ident + ".zip"
	src := "assh/" + ident + ".zip"

	var qN *qiniu.Qiniu
	if qN, err = getQiniu(); err != nil {
		log.Fatal(err)
		return
	}
	err = qN.Download(src, dst)
	if err != nil {
		log.Fatal(err)
		return
	}
	err = Unzip(dst, assh.GetDbPath())
	if err != nil {
		log.Fatal(err)
		return
	}
	os.Remove(dst)
	fmt.Println("sync download success.")
	return
}

// 导出数据
func ExportData(c *cli.Context) (err error) {
	var export = c.Args().First()
	if export == "" {
		export = "./output.yml"
	}
	export = kiris.RealPath(export)

	Assh := assh.NewAssh()
	data := kiris.NewYaml([]byte(""))

	for gn, g := range Assh.List() {
		for n, v := range g {
			s := server{
				Name:     v.Name,
				Host:     v.Host,
				Port:     v.Port,
				User:     v.User,
				Password: v.Password,
				Remark:   v.Remark,
			}

			if v.PemKey != "" {
				v.PemKey = kiris.RealPath(v.PemKey)
				s.PemKey = map[string]string{
					//"path":    v.PemKey,
					"private": "",
					"public":  "",
				}
				if pri, e := kiris.FileGetContents(v.PemKey); e == nil && len(pri) > 0 {
					s.PemKey["private"] = string(pri)
				}
				if pub, e := kiris.FileGetContents(v.PemKey + ".pub"); e == nil && len(pub) > 0 {
					s.PemKey["public"] = string(pub)
				}
			}
			name := n
			if gn != "" {
				name = gn + "." + name
			} else {
				name = "nil" + "." + name
			}
			if err = data.Set(name, s); err != nil {
				return
			}
		}
	}
	data.SaveAs(export)
	return
}

// 导入数据
func ImportData(c *cli.Context) (err error) {
	name := c.Args().First()
	if name == "" {
		name = "./servers.yml"
	}
	Assh := assh.NewAssh()

	name = kiris.RealPath(name)
	yaml := kiris.NewYamlLoad(name)
	servers := yaml.Get("").(map[string]interface{})
	for gn, gs := range servers {
		g := gs.(map[string]interface{})
		for n, ss := range g {
			name := n
			if gn != "nil" {
				name = gn + "." + name
			}

			fmt.Println("import < ", name)
			s := ss.(map[string]interface{})
			ser := assh.Server{User: "root", Port: 22}
			kiris.ConverStruct(ss.(map[string]interface{}), &ser, "json")

			if ser.Name == "" {
				ser.Name = name
			}

			if _pem, ok := s["pemkey"]; ok {
				pem := _pem.(map[string]interface{})

				if _path, ok := pem["path"]; ok {
					path := kiris.RealPath(_path.(string))
					if kiris.FileExists(path) {
						ser.PemKey = path
					}
				}
				tmp := "/tmp/_assh_pemkey_" + name
				if pri, ok := pem["private"]; ok && pri.(string) != "" {
					if err = kiris.FilePutContents(tmp, pri.(string), 0); err != nil {
						return
					}
					ser.PemKey = tmp
				}
				if pub, ok := pem["public"]; ok && pub.(string) != "" {
					kiris.FilePutContents(tmp+".pub", pub.(string), 0)
				}
			}
			if !c.IsSet("f") {
				if s := Assh.Get(name); s != nil {
					fmt.Printf("The server %s is exists, do you sure cover it? [y/n]:", name)
					var yes string
					fmt.Scanln(&yes)
					if "Y" != strings.ToUpper(yes) {
						continue
					}
				}
			}
			Assh.Set(name, ser)
			fmt.Println("imported: ", name)
		}
	}
	return
}

func getQiniu() (qn *qiniu.Qiniu, err error) {
	accessKey, secretKey, bucket := assh.GetQiniuAccessKey()
	if accessKey == "" || secretKey == "" || bucket == "" {
		err = fmt.Errorf("Assh sync > Undefined the sync configuration, please use `assh sync account` to set AccessKey and SecretKey and Bucket first")
		return
	}
	qn = qiniu.New(accessKey, secretKey, bucket)
	return
}

func lookupShortFlag(c *cli.Context, flag string) interface{} {
	var value interface{}
	flag = "-" + flag
	for i, argv := range c.Args() {
		if argv == flag {
			value = c.Args().Get(i + 1)
		}
	}
	return value
}
