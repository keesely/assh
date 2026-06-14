package port

import (
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHSession 定义 SSH 会话操作接口。
// 提供交互式 Shell、远程命令执行、以及带输出的命令执行三种模式。
type SSHSession interface {
	// Shell 启动交互式 Shell 会话，将本地终端连接到远程服务器。
	Shell(client *ssh.Client) error
	// Run 在远程服务器上执行单条命令，输出直接写入本地标准输出和标准错误。
	Run(client *ssh.Client, cmd string) error
	// RunWithOutput 在远程服务器上执行命令，将命令输出作为字符串返回。
	RunWithOutput(client *ssh.Client, cmd string) (string, error)

	// RunWithTimeout 在远程服务器上执行命令，带超时控制。
	// 超时后命令将被强制终止。
	RunWithTimeout(client *ssh.Client, cmd string, timeout time.Duration) error
	// RunWithOutputTimeout 在远程服务器上执行命令，带超时控制，返回 stdout 输出。
	RunWithOutputTimeout(client *ssh.Client, cmd string, timeout time.Duration) (string, error)
	// RunWithStderr 在远程服务器上执行命令，分离返回 stdout 和 stderr。
	RunWithStderr(client *ssh.Client, cmd string) (stdout, stderr string, err error)
}
