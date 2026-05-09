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

// DefaultTimeout 默认 SSH 连接建立超时时间
const DefaultTimeout = 10 * time.Second

type Connector struct{}

func NewConnector() *Connector {
	return &Connector{}
}

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

	// 启动 Keepalive 心跳（从 Server.Options["keepalive"] 读取间隔，秒）
	if interval := c.getKeepalive(server); interval > 0 {
		go c.keepAlive(client, time.Duration(interval)*time.Second)
	}

	return client, nil
}

// keepAlive 定期发送 SSH keepalive 请求，检测连接是否存活
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

func (c *Connector) Close(client *ssh.Client) error {
	if client != nil {
		return client.Close()
	}
	return nil
}

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

func (c *Connector) getHostKeyCallback(server *domain.Server) ssh.HostKeyCallback {
	if skip, ok := server.Options["insecure_skip_hostkey"]; ok {
		if v, ok := skip.(bool); ok && v {
			return ssh.InsecureIgnoreHostKey()
		}
	}
	return ssh.InsecureIgnoreHostKey()
}

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
