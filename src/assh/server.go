package assh

import (
	"fmt"
	"github.com/keesely/kiris"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"net"
	"os"
	//"os/signal"
	//"runtime"
	"strconv"
	//"syscall"
	"time"
)

type Server struct {
	Name     string                 `json:"name"`
	Host     string                 `json:"host"`
	Port     int                    `json:"port"`
	User     string                 `json:"user"`
	Password string                 `json:"password"`
	PemKey   string                 `json:"key"`
	Remark   string                 `json:"remark"`
	Options  map[string]interface{} `json:"options"`
	//GroupName  string                 `json:group_name`
	termWidth              int
	termHeight             int
	command, commandOutput string
}

type SSHConfig struct {
	Addr   string
	Port   int
	Config *ssh.ClientConfig
}

func (this *Server) getAuth() ([]ssh.AuthMethod, error) {
	var sshs []ssh.AuthMethod

	if "" != this.Password {
		sshs = append(sshs, ssh.Password(this.Password))
		fmt.Println("Password Login")
	}
	pubKey, _ := sshPemKey(this.PemKey, this.Password)
	sshs = append(sshs, pubKey)
	return sshs, nil
}

func (this *Server) SSHConfig() (*SSHConfig, error) {
	auth, err := this.getAuth()
	if err != nil {
		return nil, err
	}

	port := kiris.Ternary(0 == this.Port, 22, this.Port).(int)
	addr := this.Host + ":" + strconv.Itoa(port)
	return &SSHConfig{
		Addr: addr,
		Port: port,
		Config: &ssh.ClientConfig{
			User: this.User,
			Auth: auth,
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				return nil
			},
			//Timeout: 0,
		},
	}, nil
}

func (this *Server) SSHClient() (*ssh.Client, error) {
	cnf, err := this.SSHConfig()
	if err != nil {
		return nil, err
	}
	fmt.Println("Connection: ", cnf.Addr)
	return ssh.Dial("tcp", cnf.Addr, cnf.Config)
}

func (this *Server) Connection() error {
	client, err := this.SSHClient()
	if err != nil {
		check(err, " assh > dial")
		return fmt.Errorf("Assh: Connection fail: unable to authenticate \n")
	}

	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		check(err, "assh > create session")
		return fmt.Errorf("Assh: Create SESSION fail: %s \n", err.Error())
	}
	defer session.Close()

	if this.command != "" {
		buf, err := session.CombinedOutput(this.command)
		this.commandOutput = string(buf)
		return err
	}

	fd := int(os.Stdin.Fd())
	oldState, err := terminal.MakeRaw(fd)
	if err != nil {
		check(err, "assh > create session(fd)")
		return fmt.Errorf("Assh: Create SESSION(fd) fail: %s \n", err.Error())
	}
	defer terminal.Restore(fd, oldState)

	stopKeepAliveLoop := this.startKeepAliveLoop(session)
	defer close(stopKeepAliveLoop)

	if err = this.stdIO(session); err != nil {
		check(err, "assh > std I/O")
		return fmt.Errorf("Assh: Std I/O fail: %s \n", err.Error())
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	this.termWidth, this.termHeight, _ = terminal.GetSize(fd)
	termType := kiris.GetEnv("TERM", "xterm-256color").(string)

	if err = session.RequestPty(termType, this.termHeight, this.termWidth, modes); err != nil {
		check(err, "assh > request tty")
		return fmt.Errorf("Assh: Request TTY fail: %s \n", err.Error())
	}

	listenWindowChange(session, fd)

	if err = session.Shell(); err != nil {
		check(err, "assh > exec Shell")
		return fmt.Errorf("Assh: exec shell fail: %s \n", err.Error())
	}

	_ = session.Wait()
	return nil
}

// 心跳包
func (this *Server) startKeepAliveLoop(session *ssh.Session) chan struct{} {
	term := make(chan struct{})
	go func() {
		for {
			select {
			case <-term:
				return
			default:
				if val, ok := this.Options["ServerAliveInterval"]; ok && val != nil {
					_, e := session.SendRequest("keepalive@bbr", true, nil)
					if e != nil {
						check(e, "assh > keepAliveLoop")
					}
					t := time.Duration(this.Options["ServerAliveInterval"].(float64))
					time.Sleep(time.Second * t)
				} else {
					return
				}
			}
		}
	}()
	return term
}

// 重定向标准输入输出
func (this *Server) stdIO(session *ssh.Session) error {
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	session.Stdout = os.Stdout
	return nil
}

func sshPemKey(key, passwd string) (ssh.AuthMethod, error) {
	if key == "" {
		key = "~/.ssh/id_rsa"
	}
	keyPath := kiris.RealPath(key)
	fmt.Println("load pem key> ", keyPath)
	pemBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	var signer ssh.Signer
	if passwd != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(passwd))
	} else {
		signer, err = ssh.ParsePrivateKey(pemBytes)
	}
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

func (this *Server) Command(cmd string) {
	this.command = cmd
}

func (this *Server) CombinedOutput() string {
	return this.commandOutput
}
