// Package ssh 提供 SSH 连接与会话的基础设施实现。
//
// 实现 port.SSHConnector 和 port.SSHSession 接口。
// 连接支持 SSH Agent、密钥文件和密码三种认证方式，
// 以及可配置的 Keepalive 心跳检测。
package ssh

import (
	"fmt"
	"net"
	"os"
	"time"

	"assh/asshc/domain"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// DefaultTimeout 默认 SSH 连接建立超时时间（10 秒）。
const DefaultTimeout = 10 * time.Second

// Connector 实现 port.SSHConnector 接口，负责 SSH 连接的建立和关闭。
// 认证尝试顺序：SSH Agent → 密钥文件 → 密码。
type Connector struct{}

// NewConnector 创建 Connector 实例。
func NewConnector() *Connector {
	return &Connector{}
}

// Connect 根据服务器配置建立 SSH 连接。
// 自动选择认证方式、配置 HostKey 校验和 Keepalive 心跳。
func (c *Connector) Connect(server *domain.Server) (*ssh.Client, error) {
	addr := net.JoinHostPort(server.Host, fmt.Sprintf("%d", server.Port))

	sshCfg := &ssh.ClientConfig{
		User:            server.User,
		Auth:            c.authMethods(server),
		HostKeyCallback: c.getHostKeyCallback(server),
		Timeout:         DefaultTimeout,
	}

	client, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial failed: %w", err)
	}

	// 从 Options["keepalive"] 读取心跳间隔（秒），大于 0 时启动协程
	if interval := c.getKeepalive(server); interval > 0 {
		go c.keepAlive(client, time.Duration(interval)*time.Second)
	}

	return client, nil
}

// keepAlive 定期发送 SSH keepalive@openssh.com 请求，检测连接是否存活。
// 如果 keepalive 发送失败（连接已断开），自动退出协程。
func (c *Connector) keepAlive(client *ssh.Client, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
		if err != nil {
			return
		}
	}
}

// Close 关闭 SSH 连接，client 为 nil 时不做操作。
func (c *Connector) Close(client *ssh.Client) error {
	if client != nil {
		return client.Close()
	}
	return nil
}

// authMethods 根据服务器配置，按优先级收集可用的认证方式。
// 认证顺序：SSH Agent → 密钥文件 → 密码。
func (c *Connector) authMethods(server *domain.Server) []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	if m := c.tryAgent(); m != nil {
		methods = append(methods, m)
	}

	if server.Auth != nil && server.Auth.KeyFile != "" {
		if m := c.tryKeyFile(server.Auth.KeyFile); m != nil {
			methods = append(methods, m)
		}
	}

	if server.Auth != nil && server.Auth.Password != "" {
		methods = append(methods, ssh.Password(server.Auth.Password))
	}

	return methods
}

// tryAgent 尝试使用 SSH Agent（通过 SSH_AUTH_SOCK 环境变量）进行认证。
// 如果环境变量未设置或连接失败，返回 nil。
func (c *Connector) tryAgent() ssh.AuthMethod {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil
	}
	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers)
}

// tryKeyFile 尝试使用指定路径的私钥文件进行认证。
// 如果文件读取失败或密钥解析失败，返回 nil。
func (c *Connector) tryKeyFile(keyFile string) ssh.AuthMethod {
	key, err := os.ReadFile(keyFile)
	if err != nil {
		return nil
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(signer)
}

// getHostKeyCallback 返回 HostKey 校验回调函数。
// 如果 Options["insecure_skip_hostkey"] 为 true，则跳过校验。
// 当前默认行为为跳过校验（InsecureIgnoreHostKey）。
func (c *Connector) getHostKeyCallback(server *domain.Server) ssh.HostKeyCallback {
	if skip, ok := server.Options["insecure_skip_hostkey"]; ok {
		if v, ok := skip.(bool); ok && v {
			return ssh.InsecureIgnoreHostKey()
		}
	}
	return ssh.InsecureIgnoreHostKey()
}

// getKeepalive 从服务器配置中读取 Keepalive 心跳间隔（秒）。
// 返回 0 表示不启用 Keepalive。
func (c *Connector) getKeepalive(server *domain.Server) int {
	if v, ok := server.Options["keepalive"]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		}
	}
	return 0
}
