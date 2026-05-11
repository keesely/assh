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
// 认证策略：优先尝试 Agent/KeyFile/Password 的组合，如果 KeyFile 认证失败
// 且 Password 可用，自动回退到 Password-only 重新连接。
func (c *Connector) Connect(server *domain.Server) (*ssh.Client, error) {
	addr := net.JoinHostPort(server.Host, fmt.Sprintf("%d", server.Port))

	// First attempt: try all available auth methods
	primaryMethods := c.authMethods(server)
	sshCfg := &ssh.ClientConfig{
		User:            server.User,
		Auth:            primaryMethods,
		HostKeyCallback: c.getHostKeyCallback(server),
		Timeout:         DefaultTimeout,
	}

	client, err := ssh.Dial("tcp", addr, sshCfg)
	if err == nil {
		c.setupKeepalive(client, server)
		return client, nil
	}

	// Fallback: if key auth was attempted and failed, and password is available,
	// try a second connection with password-only auth.
	if c.hasKeyAndPassword(server) {
		fallbackCfg := &ssh.ClientConfig{
			User:            server.User,
			Auth:            []ssh.AuthMethod{ssh.Password(server.Auth.Password)},
			HostKeyCallback: c.getHostKeyCallback(server),
			Timeout:         DefaultTimeout,
		}

		client2, err2 := ssh.Dial("tcp", addr, fallbackCfg)
		if err2 == nil {
			c.setupKeepalive(client2, server)
			return client2, nil
		}
	}

	return nil, fmt.Errorf("ssh dial failed: %w", err)
}

// hasKeyAndPassword 检查服务器是否同时配置了密钥文件和密码。
func (c *Connector) hasKeyAndPassword(server *domain.Server) bool {
	return server.Auth != nil && server.Auth.KeyFile != "" && server.Auth.Password != ""
}

// setupKeepalive 从服务器配置中读取 Keepalive 心跳间隔（秒），大于 0 时启动协程。
func (c *Connector) setupKeepalive(client *ssh.Client, server *domain.Server) {
	if interval := c.getKeepalive(server); interval > 0 {
		go c.keepAlive(client, time.Duration(interval)*time.Second)
	}
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
// 如果密钥和密码同时配置，初始尝试使用全部方法；若密钥认证失败，
// Connect 会自动进行 Password-only 回退（见 Connect 方法）。
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
// 如果环境变量未设置、连接失败、或 agent 中没有加载任何 identities，返回 nil。
// 注意：返回空的 PublicKeysCallback 会干扰 Go SSH 的认证循环，导致后续的
// keyfile 和 password 方法被跳过（所有 publickey 方法被认为是已尝试过的）。
func (c *Connector) tryAgent() ssh.AuthMethod {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil
	}

	agentClient := agent.NewClient(conn)

	// 预检查 agent 是否加载了任何 identities。
	// 空壳 PublicKeysCallback（0 个 signers）会导致 Go SSH 认证循环提前退出，
	// 使后续的 keyfile 和 password 认证方法不会被尝试。
	signers, err := agentClient.Signers()
	if err != nil || len(signers) == 0 {
		return nil
	}

	return ssh.PublicKeysCallback(agentClient.Signers)
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
