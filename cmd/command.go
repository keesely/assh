// command.go kee > 2019/12/02

package cmd

import (
	assh "assh/asshc"
	"assh/cmd/qiniu"
	"assh/log"
	"fmt"
	"github.com/keesely/kiris"
	"github.com/keesely/kiris/hash"
	"github.com/urfave/cli"
	"os"
	//"reflect"
	"time"
	//"strings"
)

type App struct {
	app *cli.App
}

var commonFlags = []cli.Flag{
	cli.StringFlag{Name: "H", Value: "", Usage: "server host"},
	cli.IntFlag{Name: "p", Value: 0, Usage: "bind address port"},
	cli.StringFlag{Name: "l", Value: "", Usage: "login name"},
	cli.StringFlag{Name: "P", Value: "", Usage: "login password"},
	cli.StringFlag{Name: "k", Value: "", Usage: "automatic login public key"},
}

var commands = []cli.Command{
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
		Name:   "push",
		Usage:  "sftp push file to remote server",
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
}

func NewCli() *App {
	app := cli.NewApp()
	app.Name = "Assh - An SSH Client"
	app.Usage = ""
	app.Version = "0.0.1"
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

		fmt.Printf("vf: %v cArgs: %d\n", vf, len(c.Args()))
		if vf || 0 < len(c.Args()) {
			return Connection(c)
		}
		return

	}
}

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
