// server.go kee > 2019/12/02

package cmd

import (
	assh "assh/asshc"
	keygen "assh/asshc/keygen"
	"assh/log"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/keesely/kiris"
	"github.com/keesely/kiris/hash"
	"github.com/urfave/cli"
)

func printListServer(data map[string]map[string]*assh.Server) {
	fmt.Println(kiris.StrPad("", "=", 160, 0))
	fmt.Printf("  %-20s | %-20s | %-50s | %-50s \n", "Group Name", "Server Name", "Server Host", "Remark")
	fmt.Println(kiris.StrPad("", "-", 160, 0))
	var i = 0
	for g, ss := range data {
		for n, s := range ss {
			i++
			var color = fmt.Sprintf("\033[0m")
			if i%4 == 0 {
				color = fmt.Sprintf("\033[1m")
			}

			sInfo := fmt.Sprintf("%s@%s:%d (%s)",
				s.User,
				s.Host,
				s.Port,
				kiris.Ternary(s.Password != "",
					"passwd:yes",
					kiris.Ternary(s.PemKey != "", "pub key:yes", "passwd:no").(string),
				).(string),
			)
			remark := kiris.Ternary(s.Remark != "", s.Remark, "\033[0;37m (no remark) \033[0m")

			fmt.Printf("> "+color+"%-20s\033[0m | "+color+"%-20s\033[0m | "+color+"%-50s\033[0m | %s \n", g, n, sInfo, remark)
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
	for i, k := range []string{"Name", "Host", "Port", "User", "Password", "PemKey", "Remark"} {
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
		if !c.IsSet("f") {
			fmt.Printf("The server %s is exists, do you sure cover it? [y/n]:", name)
			var yes string
			fmt.Scanln(&yes)
			if "Y" != strings.ToUpper(yes) {
				return
			}
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

func BatchCommand(c *cli.Context) (err error) {
	args := c.Args()
	Assh := assh.NewAssh()
	cmd := args.Get(1)
	if f := c.String("f"); f != "" {
		if _c, ferr := kiris.FileGetContents(f); ferr == nil {
			cmd = string(_c)
		}
	}

	if name := args.First(); "" != name && "" != cmd {
		defer timeCost(time.Now(), "BATCH COMMAND: ")
		c.Set("c", cmd)
		ch := make(chan string)

		var chConnect = func(s *assh.Server, cmd string, ch chan string) {
			_connection(s, cmd)
			ch <- s.Name
		}
		if g := Assh.GetGroup(name); g != nil {
			for _, s := range g {
				s.Name = fmt.Sprintf("%s@%s:%d", s.User, s.Host, s.Port)
				go chConnect(s, cmd, ch)
			}

			for i := 0; i < len(g); i++ {
				println("connection closed.", <-ch)
			}
		}
	}

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
	if _c := lookupShortFlag(c, "c"); _c != nil || c.IsSet("c") {
		if _c != nil {
			cmd = _c.(string)
		} else {
			cmd = c.String("c")
		}
	}
	_connection(s, cmd)
}

func _connection(s *assh.Server, cmd string) {
	if cmd != "" {
		s.Command(cmd)
	}

	defer timeCost(time.Now())

	host := fmt.Sprintf("%s@%s:%d", s.User, s.Host, s.Port)
	passwd := kiris.Ternary(s.PemKey != "",
		"(pemKey: yes)",
		fmt.Sprintf("(password: %s)", kiris.Ternary(s.Password != "", "yes", "no")),
	)

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

	defer timeCost(time.Now(), "SFTP PUSH")
	name := args.First()
	bufSize := c.Int("b")
	if g := assh.NewAssh().GetGroup(name); g != nil {
		ch := make(chan string)
		if 0 >= bufSize {
			bufSize = 2048
		}
		var chPush = func(s *assh.Server, localPath []string, remotePath string, ch chan string, bufSize int) {
			if err := s.ScpPushFiles(localPath, remotePath, bufSize); err != nil {
				log.Errorf("sftp push[%s] fail: %s", s.Name, err.Error())
			}
			ch <- fmt.Sprintf("[%s] pushed.\n", s.Name)
		}
		for _, s := range g {
			go chPush(s, localPath, remotePath, ch, bufSize)
		}
		for i := 0; i < len(g); i++ {
			print(<-ch)
		}
		return
	}

	if s := getSshClient(c); s != nil {
		if err := s.ScpPushFiles(localPath, remotePath, bufSize); err != nil {
			log.Error("sftp push fail: ", err.Error())
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

	name := args.First()
	if g := assh.NewAssh().GetGroup(name); g != nil {
		ch := make(chan string)
		var chPull = func(s *assh.Server, remotePath []string, localPath string, ch chan string) {
			if err := s.ScpPullFiles(remotePath, localPath); err != nil {
				log.Errorf("sftp push[%s] fail: %s", s.Name, err.Error())
			}
			ch <- fmt.Sprintf("[%s] pushed.\n", s.Name)
		}
		for _, s := range g {
			go chPull(s, remotePath, localPath, ch)
		}
		for i := 0; i < len(g); i++ {
			println(<-ch)
		}
		return
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
		Assh := assh.NewAssh()
		var ss []*assh.Server

		if g := Assh.GetGroup(name); g != nil {
			for _, s := range g {
				ss = append(ss, s)
			}
		} else if s := Assh.Get(name); s != nil {
			ss = append(ss, s)
		}
		if len(ss) > 0 {
			for _, s := range ss {
				pemKey := "/tmp/" + hash.Md5(name)
				saveKey(pemKey, private, public)
				sshCopyId(s, public)
				s.PemKey = pemKey
				Assh.Set(name, *s)
			}
		}
	}

	saveKey(saveFs, private, public)
	return
}

func PingServers(c *cli.Context) (err error) {
	ss := assh.NewAssh()
	pingServersPrint(ss.List())
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

func timeCost(start time.Time, prefix ...string) {
	tc := time.Since(start)
	px := strings.Join(prefix, "")
	fmt.Printf("> %s time cost: %v\n", px, tc)
}

func pingServersPrint(data map[string]map[string]*assh.Server) {
	fmt.Println(kiris.StrPad("", "=", 160, 0))
	fmt.Printf(" %-20s | %-50s | %-50s \n", "Server Name", "Server Host", "Result")
	fmt.Println(kiris.StrPad("", "-", 160, 0))

	var ch = make(chan string)
	var chn = make(chan int)
	var printServer = func(color, remark string, s *assh.Server) string {
		sInfo := fmt.Sprintf("%s@%s:%d (%s)",
			s.User,
			s.Host,
			s.Port,
			kiris.Ternary(s.Password != "",
				"passwd:yes",
				kiris.Ternary(s.PemKey != "", "pub key:yes", "passwd:no").(string),
			).(string),
		)
		return fmt.Sprintf("> "+color+"%-20s\033[0m | "+color+"%-50s\033[0m | "+color+"%s\033[0m", s.Name, sInfo, remark)
	}
	var chPrintServer = func(n int, s *assh.Server) {
		var color = fmt.Sprintf("\033[0m")
		start := time.Now()

		if _, err := s.SSHActive(); err != nil {
			color = fmt.Sprintf("\033[31m")
			ch <- printServer(color, fmt.Sprintf(err.Error()), s)
			chn <- n
		} else {
			tc := time.Since(start)
			ch <- printServer(color, fmt.Sprintf("time cost: %v", tc), s)
			chn <- n
		}
	}

	var x = 0
	for _, ss := range data {
		for _, s := range ss {
			go chPrintServer(x, s)
			x++
		}
	}

	for i := 0; i < x; i++ {
		println(<-ch)
		fmt.Printf("Scanning: %d/%d\r", i, x)
	}
	fmt.Println(kiris.StrPad("", "=", 160, 0))

}

// 处理转发
func Proxy(c *cli.Context) (err error) {
	port := c.String("d")
	host := c.String("i")
	if port == "" {
		port = "1080"
	}

	defer timeCost(time.Now(), "SSH PROXY")

	args := c.Args()
	name := args.First()
	if server := assh.NewAssh().Get(name); server != nil {
		server.Proxy(host, port)
	}
	return nil
}

// 远程主机端口转发: ProtForwarding
func ProxyHost(c *cli.Context) (err error) {
	rhost := c.String("H")
	rport := c.String("P")
	port := c.String("d")
	host := c.String("i")
	if port == "" {
		port = "1080"
	}

	defer timeCost(time.Now(), "SSH PROXY Host")

	args := c.Args()
	name := args.First()
	if server := assh.NewAssh().Get(name); server != nil {
		server.PortForwarding(host, port, rhost, rport)
	}
	return nil
}

// 监听远程主机端口转发: LocalProxy
func LocalProxy(c *cli.Context) (err error) {
	rhost := c.String("H")
	rport := c.String("P")
	port := c.String("d")
	host := c.String("i")
	if port == "" {
		port = "1080"
	}

	defer timeCost(time.Now(), "SSH PROXY Host")

	args := c.Args()
	name := args.First()
	if server := assh.NewAssh().Get(name); server != nil {
		server.LocalProxy(rhost, rport, host, port)
		//server.LocalProxyWithRetry(rhost, rport, host, port)
	}
	return nil
}
