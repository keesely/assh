// server.go kee > 2019/12/02

package cmd

import (
	assh "assh/asshc"
	keygen "assh/asshc/keygen"
	"assh/log"
	"fmt"
	"github.com/keesely/kiris"
	"github.com/keesely/kiris/hash"
	"github.com/urfave/cli"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func printListServer(data map[string]map[string]*assh.Server) {
	fmt.Println(kiris.StrPad("", "=", 160, 0))
	fmt.Printf("  %-20s | %-20s | %-50s | %-50s \n", "Group Name", "Server Name", "Server Host", "Remark")
	fmt.Println(kiris.StrPad("", "-", 160, 0))
	for g, ss := range data {
		for n, s := range ss {
			sInfo := fmt.Sprintf("%s@%s:%d (%s)",
				s.User,
				s.Host,
				s.Port,
				kiris.Ternary(s.Password != "",
					"passwd:yes",
					kiris.Ternary(s.PemKey != "", "pub key:yes", "passwd:no").(string),
				).(string),
			)
			remark := kiris.Ternary(s.Remark != "", s.Remark, " (no remark) ")
			fmt.Printf("> %-20s | %-20s | %-50s | %s \n", g, n, sInfo, remark)
		}
	}
	fmt.Println(kiris.StrPad("", "=", 160, 0))
}

func ListServer(c *cli.Context) error {
	ss := assh.NewAssh()
	printListServer(ss.List())
	return nil
}

func SearchServer(c *cli.Context) error {
	name := c.Args().First()
	data := assh.NewAssh().Search(name)
	printListServer(data)
	return nil
}

func printServerInfo(s *assh.Server) {
	fmt.Println(kiris.StrPad("", "=", 100, 0))
	fmt.Println(kiris.StrPad(" Server Information Detail ", "+", 100, kiris.KIRIS_STR_PAD_BOTH))
	fmt.Println(kiris.StrPad("", "-", 100, 0))
	ss := reflect.ValueOf(s).Elem()
	for i, k := range []string{"Name", "Host", "Port", "User", "Password", "PemKey"} {
		fmt.Printf("%20s:   %v\n", "Server "+k, ss.Field(i))
	}
	fmt.Println(kiris.StrPad("", "=", 100, 0))
}

func InfoServer(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return nil
	}

	ss := assh.NewAssh()
	if s := ss.Get(name); s != nil {
		printServerInfo(s)
	} else {
		fmt.Printf("Server [%s] not found\n", name)
		log.Infof("show info: Server [%s] not found\n", name)
	}
	return nil
}

func SetServer(c *cli.Context) (err error) {
	var server = &assh.Server{User: "root", Port: 22}

	ss := assh.NewAssh()
	name := c.Args().First()
	host := c.Args().Get(1)
	if s := ss.Get(name); s != nil {
		server = s
		fmt.Printf("The server %s is exists, do you sure cover it? [y/n]:", name)
		var yes string
		fmt.Scanln(&yes)
		if "Y" != strings.ToUpper(yes) {
			return
		}
	}
	if host != "" {
		if h, u, p := parseHostString(host); h != "" {
			server.Host = h
			if p > 0 {
				server.Port = p
			}
			if u != "" {
				server.User = u
			}
		}
	}

	if h := c.String("H"); h != "" {
		server.Host = h
	}
	if server.Host == "" {
		log.Fatal("set server: hostname is nil")
	}

	if u := c.String("l"); u != "" {
		server.User = u
	}
	if p := c.Int("p"); p > 0 {
		server.Port = p
	}
	if P := c.String("P"); P != "" {
		server.Password = P
	}
	if k := c.String("k"); k != "" {
		server.PemKey = k
	}
	if R := c.String("R"); R != "" {
		server.Remark = R
	}

	server.Name = name

	ss.Set(name, *server)
	fmt.Printf("Server [%s] success.\n", name)
	printServerInfo(server)
	return
}

func MoveServer(c *cli.Context) (err error) {
	from := c.Args().First()
	to := c.Args().Get(1)
	assh.NewAssh().Move(from, to)
	return nil
}

func RemoveServer(c *cli.Context) (err error) {
	name := c.Args().First()
	ss := assh.NewAssh()
	if c.IsSet("g") {
		if name == "" {
			fmt.Println("group name is nil")
		}
		g := ss.GetGroup(name)
		if g == nil {
			fmt.Println("group no exists.")
			return
		}

		for _, s := range g {
			delServer(ss, s.Name, c)
		}
		return
	}
	if name != "" {
		delServer(ss, name, c)
	} else {
		ss.List()
		fmt.Printf("witch do you want to delete: ")
		fmt.Scanln(&name)
		if name != "" {
			delServer(ss, name, c)
		}
		return
	}
	return
}

func delServer(ss *assh.Assh, name string, c *cli.Context) {
	if s := ss.Get(name); s != nil {
		var yes string

		if !c.IsSet("f") {
			fmt.Printf("Are you sure (delete %s) [y/n]: ", name)
			fmt.Scanln(&yes)
		} else {
			yes = "Y"
		}
		if "Y" == strings.ToUpper(yes) {
			ss.Del(name)
			log.Infof("Remove Server [%s]\n", name)
		}
	} else {
		fmt.Printf("服务器(%s)不存在于服务器列表中.\n", name)
	}
}

func Connection(c *cli.Context) (err error) {
	s := &assh.Server{User: "root", Port: 22}
	args := c.Args()

	name := args.First()
	if "" != name && "-c" != name {
		if server := assh.NewAssh().Get(name); server != nil {
			connection(server, c)
			return
		}
		if h, u, p := parseHostString(name); h != "" {
			s.Host = h
			if u != "" {
				s.User = u
			}
			if p > 0 {
				s.Port = p
			}
		}
	}

	if h := c.String("H"); h != "" {
		s.Host = h
	}
	if s.Host == "" {
		log.Fatal("login server: hostname is nil")
	}

	if u := c.String("l"); u != "" {
		s.User = u
	}
	if p := c.Int("p"); p > 0 {
		s.Port = p
	}
	if P := c.String("P"); P != "" {
		s.Password = P
		fmt.Printf("Password: %s", P)
	}
	if k := c.String("k"); k != "" {
		s.PemKey = k
	}
	s.Name = fmt.Sprintf("%s@%s:%d", s.User, s.Host, s.Port)
	connection(s, c)
	return
}

func parseHostString(s string) (host, user string, port int) {
	if i := strings.Index(s, "@"); i > 0 {
		user = s[:i]
		s = s[i+1:]
	}
	if i := strings.Index(s, ":"); i > 0 {
		if p, _ := strconv.Atoi(s[i+1:]); p > 0 {
			port = p
		}
		s = s[:i]
	}

	host = s
	return
}

func getSshClient(c *cli.Context) *assh.Server {
	s := &assh.Server{User: "root", Port: 22}
	args := c.Args()

	name := args.First()
	if "" != name && "-c" != name {
		if server := assh.NewAssh().Get(name); server != nil {
			return server
		}
		if h, u, p := parseHostString(name); h != "" {
			s.Host = h
			if u != "" {
				s.User = u
			}
			if p > 0 {
				s.Port = p
			}
		}
	}

	if h := c.String("H"); h != "" {
		s.Host = h
	}
	if s.Host == "" {
		log.Fatal("login server: hostname is nil")
	}

	if u := c.String("l"); u != "" {
		s.User = u
	}
	if p := c.Int("p"); p > 0 {
		s.Port = p
	}
	if P := c.String("P"); P != "" {
		s.Password = P
	}
	if k := c.String("k"); k != "" {
		s.PemKey = k
	}
	s.Name = fmt.Sprintf("%s@%s:%d", s.User, s.Host, s.Port)

	if "" == s.Password && "" == s.PemKey {
		// 使用默认公钥
		if c.IsSet("k") {
			s.PemKey = "~/.ssh/id_rsa"
		} else {
			fmt.Printf("%s@%s's password: ", s.User, s.Host)
			fmt.Scanln(&s.Password)
		}
	}
	return s
}

func connection(s *assh.Server, c *cli.Context) {
	// 执行远程命令
	var cmd string
	if _c := lookupShortFlag(c, "c"); _c != nil {
		cmd = _c.(string)
		s.Command(cmd)
	}

	host := fmt.Sprintf("%s@%s:%d", s.User, s.Host, s.Port)
	passwd := kiris.Ternary(s.PemKey != "",
		"(pemKey: yes)",
		fmt.Sprintf("(password: %s)", kiris.Ternary(s.Password != "", "yes", "no")),
	)

	defer timeCost(time.Now())

	fmt.Printf("> connection: %s %s \n", host, passwd)
	log.Infof("connection server [%s]\n", host)

	err := s.Connection()
	if err != nil {
		log.Error(err.Error())
	}
	log.Infof("%s connection closed.", host)

	if cmd != "" {
		cout := s.CombinedOutput()
		fmt.Printf("\n> Result (%s): \n%s\n", fmt.Sprintf("%s -> '%s'", s.Name, host), cout)
		log.Infof("Executive (%s -> '%s'):"+
			"\n==============================================================================="+
			"\nCommand > %s"+
			"\n-------------------------------------------------------------------------------"+
			"\n%s"+
			"\n==============================================================================="+
			"\n\n", s.Name, host, cmd, cout)
	}
}

func ScpPush(c *cli.Context) (err error) {
	var (
		localPath  []string
		remotePath string
	)

	args := c.Args()
	if len(args) > 2 {
		localPath = args[1 : len(args)-1]
		remotePath = args[len(args)-1]
	} else {
		localPath = args[1:]
		remotePath = "~/"
	}

	if 0 >= len(localPath) {
		fmt.Println("sftp push fail: the local file/directory is empty")
	}

	if s := getSshClient(c); s != nil {
		if err := s.ScpPushFiles(localPath, remotePath); err != nil {
			log.Errorf("sftp push [%s] fail: ", err.Error())
		}
		return
	}
	fmt.Println("sftp push fail: invalid server.")
	return
}

func ScpPull(c *cli.Context) (err error) {
	var (
		remotePath []string
		localPath  string
	)

	args := c.Args()
	if len(args) > 2 {
		remotePath = args[1 : len(args)-1]
		localPath = args[len(args)-1]
	} else {
		remotePath = args[1:]
		localPath = "./"
	}

	if 0 >= len(remotePath) {
		fmt.Println("sftp pull fail: the remote file/directory is nil")
	}

	if s := getSshClient(c); s != nil {
		if err := s.ScpPullFiles(remotePath, localPath); err != nil {
			log.Errorf("sftp pull [%s] fail: ", err.Error())
		}
		return
	}
	fmt.Println("sftp pull fail: invalid server.")
	return
}

func Keygen(c *cli.Context) (err error) {
	comment := c.String("c")
	saveFs := c.String("f")

	if comment == "" && c.Args().First() != "" {
		comment = fmt.Sprintf("assh@%s", c.Args().First())
	}

	key, _ := keygen.NewRsa(2048)
	public, private, _ := key.GenSSHKey(comment)

	var saveKey = func(f, private, public string) {
		kiris.FilePutContents(f, private, 0)
		kiris.FilePutContents(f+".pub", public, 0)
	}

	// 生成服务器公钥
	if name := c.Args().First(); name != "" {
		ss := assh.NewAssh()
		if s := ss.Get(name); s != nil {
			pemKey := "/tmp/" + hash.Md5(name)
			saveKey(pemKey, private, public)
			sshCopyId(s, public)
			s.PemKey = pemKey
			ss.Set(name, *s)
		}
	}
	saveKey(saveFs, private, public)
	return
}

func sshCopyId(s *assh.Server, public string) {
	script := `umask 077;` +
		`test -d ~/.ssh || mkdir ~/.ssh; echo "` + public + `"  >> ~/.ssh/authorized_keys ` +
		`&& (test -x /sbin/restorecon && /sbin/restorecon ~/.ssh ~/.ssh/authorized_keys >/dev/null 2>&1 || true)`
	s.Command(script)
	host := fmt.Sprintf("%s -> %s@%s", s.Name, s.User, s.Host)
	fmt.Printf("> connection: %s \n", host)
	log.Infof("connection server [%s]: ssh_copy_id \n", host)
	err := s.Connection()
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("connection server [%s] close: ssh_copy_id\n", host)
	if cout := s.CombinedOutput(); cout != "" {
		log.Infof("ssh_copy_id -> %s: \n> %s \n", host, cout)
	}
}

func timeCost(start time.Time) {
	tc := time.Since(start)
	fmt.Printf("> time cost: %v\n", tc)
}
