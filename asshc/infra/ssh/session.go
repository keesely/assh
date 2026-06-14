package ssh

import (
	"bytes"
	"context"
	"os"
	"time"

	"assh/asshc/port"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// Session 实现 port.SSHSession 接口，提供 SSH 会话操作的具体实现。
// 支持交互式 Shell（含终端大小自适应调整）和远程命令执行。
type Session struct{}

// 编译期接口合规检查
var _ port.SSHSession = (*Session)(nil)

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
	// 使用平台特定的实现（Unix: SIGWINCH, Windows: 占位）
	go watchResize(fd, func(w, h int) {
		session.WindowChange(h, w)
	})

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

// RunWithTimeout 在远程服务器上执行命令，带超时控制。
// 超时后命令将被强制终止。
func (s *Session) RunWithTimeout(client *ssh.Client, cmd string, timeout time.Duration) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	if err := session.Start(cmd); err != nil {
		return err
	}

	// 创建超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// 超时，强制关闭会话
		session.Close()
		return &TimeoutError{Duration: timeout}
	}
}

// RunWithOutputTimeout 在远程服务器上执行命令，带超时控制，返回 stdout 输出。
func (s *Session) RunWithOutputTimeout(client *ssh.Client, cmd string, timeout time.Duration) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var stdout bytes.Buffer
	session.Stdout = &stdout

	if err := session.Start(cmd); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case err := <-done:
		return stdout.String(), err
	case <-ctx.Done():
		session.Close()
		return stdout.String(), &TimeoutError{Duration: timeout}
	}
}

// RunWithStderr 在远程服务器上执行命令，分离返回 stdout 和 stderr。
func (s *Session) RunWithStderr(client *ssh.Client, cmd string) (stdout, stderr string, err error) {
	session, err := client.NewSession()
	if err != nil {
		return "", "", err
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	if err := session.Run(cmd); err != nil {
		return stdoutBuf.String(), stderrBuf.String(), err
	}

	return stdoutBuf.String(), stderrBuf.String(), nil
}

// TimeoutError 表示命令执行超时错误。
type TimeoutError struct {
	Duration time.Duration
}

func (e *TimeoutError) Error() string {
	return "command timed out after " + e.Duration.String()
}
