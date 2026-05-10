package ssh

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// Session 实现 port.SSHSession 接口，提供 SSH 会话操作的具体实现。
// 支持交互式 Shell（含终端大小自适应调整）和远程命令执行。
type Session struct{}

// NewSession 创建 Session 实例。
func NewSession() *Session {
	return &Session{}
}

// Shell 启动交互式 Shell 会话，将本地终端连接到远程服务器。
// 功能包括：
//   - 终端设置为原始模式（raw mode），支持全屏交互
//   - 申请 xterm-256color 类型的伪终端
//   - 监听 SIGWINCH 信号，终端窗口大小变化时自动调整远程终端
func (s *Session) Shell(client *ssh.Client) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, oldState)

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	w, h, err := term.GetSize(fd)
	if err != nil {
		w, h = 80, 24
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("xterm-256color", h, w, modes); err != nil {
		return err
	}

	if err := session.Shell(); err != nil {
		return err
	}

	// 监听终端窗口大小变化信号，实时调整远程 PTY 尺寸
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)

	go func() {
		for range sigCh {
			w, h, _ := term.GetSize(fd)
			session.WindowChange(h, w)
		}
	}()

	return session.Wait()
}

// Run 在远程服务器上以非交互模式执行命令，输出直接写入本地 stdout/stderr。
func (s *Session) Run(client *ssh.Client, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	return session.Run(cmd)
}

// RunWithOutput 在远程服务器上执行命令，将 stdout 和 stderr 合并后作为字符串返回。
func (s *Session) RunWithOutput(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	return string(output), err
}
