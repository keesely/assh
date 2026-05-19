// Package ssh 提供 SSH 连接与会话的基础设施实现。
//
// 实现 port.SSHConnector 和 port.SSHSession 接口。
// 连接支持 SSH Agent、密钥文件和密码三种认证方式，
// 以及可配置的 Keepalive 心跳检测。
package ssh

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"assh/asshc/domain"
	"assh/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

// DefaultTimeout 默认 SSH 连接建立超时时间（10 秒）。
const DefaultTimeout = 10 * time.Second

// Connector 实现 port.SSHConnector 接口，负责 SSH 连接的建立和关闭。
// 认证尝试顺序：SSH Agent → 密钥文件 → 密码。
// 支持密钥备份优先策略（data/keys/）和加密密钥自动解密。
type Connector struct {
	accountPassphrase []byte // 用于解密 data/keys/ 中加密的备份密钥
}

// NewConnector 创建 Connector 实例（不含 passphrase，用于向后兼容）。
// 密钥备份优先策略不可用，备份密钥解密不可用。
func NewConnector() *Connector {
	return &Connector{}
}

// NewConnectorWithPassphrase 创建包含 account passphrase 的 Connector 实例。
// 用于解密 data/keys/ 中使用 account password 加密的备份密钥。
// passphrase 为 nil 时行为等同于 NewConnector()。
func NewConnectorWithPassphrase(accountPassphrase []byte) *Connector {
	return &Connector{
		accountPassphrase: accountPassphrase,
	}
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

// ConnectChain 通过跳板链连接到目标服务器。
// chain 参数为有序的跳板服务器列表，从第一个依次连接到最后的目标服务器。
// 返回最终的 ssh.Client（连接到目标服务器）。
func (c *Connector) ConnectChain(target *domain.Server, chain []*domain.Server) (*ssh.Client, error) {
	if len(chain) == 0 {
		return c.Connect(target)
	}

	var parents []*ssh.Client

	// Step 1: 连接第一个跳板机
	first := chain[0]
	client, err := c.Connect(first)
	if err != nil {
		return nil, fmt.Errorf("jump[0] %s: %w", first.Host, err)
	}
	parents = append(parents, client)

	// Step 2: 依次串联后续跳板机
	for i := 1; i < len(chain); i++ {
		hop := chain[i]
		client, err = c.dialThrough(client, hop)
		if err != nil {
			c.closeAll(parents)
			return nil, fmt.Errorf("jump[%d] %s: %w", i, hop.Host, err)
		}
		parents = append(parents, client)
	}

	// Step 3: 最后连接目标
	targetClient, err := c.dialThrough(client, target)
	if err != nil {
		c.closeAll(parents)
		return nil, fmt.Errorf("target %s: %w", target.Host, err)
	}

	return targetClient, nil
}

// ChainClient 包装 SSH 客户端，包含目标连接和所有跳板机连接。
// Close 时按"目标 → 最后一跳 → ... → 第一跳"顺序级联关闭。
type ChainClient struct {
	*ssh.Client
	parents []*ssh.Client
}

// Close 级联关闭所有连接。
func (c *ChainClient) Close() error {
	// 先关闭目标连接
	c.Client.Close()
	// 再按倒序关闭所有跳板连接
	for i := len(c.parents) - 1; i >= 0; i-- {
		c.parents[i].Close()
	}
	return nil
}

// dialThrough 通过已有客户端拨号到目标服务器。
// 连接建立后自动配置 Keepalive（与直连行为一致）。
func (c *Connector) dialThrough(parent *ssh.Client, server *domain.Server) (*ssh.Client, error) {
	addr := net.JoinHostPort(server.Host, fmt.Sprintf("%d", server.Port))
	conn, err := parent.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, c.buildConfig(server))
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake %s: %w", addr, err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	c.setupKeepalive(client, server)
	return client, nil
}

// buildConfig 根据服务器配置构建 SSH ClientConfig。
func (c *Connector) buildConfig(server *domain.Server) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User:            server.User,
		Auth:            c.authMethods(server),
		HostKeyCallback: c.getHostKeyCallback(server),
		Timeout:         DefaultTimeout,
	}
}

// closeAll 关闭所有跳板连接（用于错误回滚）。
func (c *Connector) closeAll(clients []*ssh.Client) {
	for _, client := range clients {
		client.Close()
	}
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
// 认证策略：
//  1. 优先查找 data/keys/ 中的备份（由 BackupPath 计算路径）。
//     如果备份存在且能解密，优先使用备份（防止原始密钥被篡改/删除）。
//  2. 回退到原始路径。
//
// 如果设置了 accountPassphrase，会尝试用 passphrase 解密备份密钥。
// 解密失败时回退到原始路径。
func (c *Connector) tryKeyFile(keyFile string) ssh.AuthMethod {
	// 1. 优先查找 data/keys/ 中的备份
	if c.accountPassphrase != nil {
		if m := c.tryBackupKey(keyFile); m != nil {
			return m
		}
	}

	// 2. 回退到原始路径
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

// tryBackupKey 尝试从 data/keys/ 目录中加载备份密钥。
// 计算 keyFile 对应的备份路径，检查文件是否存在，
// 如果存在则尝试用 accountPassphrase 解密。
func (c *Connector) tryBackupKey(keyFile string) ssh.AuthMethod {
	// 计算备份路径（基于 keyFile 内容的 SHA256 哈希）
	backupPath := computeKeyBackupPath(keyFile)
	if backupPath == "" {
		return nil
	}

	key, err := os.ReadFile(backupPath)
	if err != nil {
		return nil
	}

	// 优先尝试用 accountPassphrase 解密
	signer, err := ssh.ParsePrivateKeyWithPassphrase(key, c.accountPassphrase)
	if err != nil {
		// 解密失败，可能是未加密的密钥，尝试无 passphrase 解析
		signer, err = ssh.ParsePrivateKey(key)
		if err != nil {
			return nil
		}
	}

	return ssh.PublicKeys(signer)
}

// computeKeyBackupPath 根据密钥文件路径计算对应的备份路径。
// 使用三层嵌套目录结构：ab/cd/{sha256(keyFile)}.pem（前4字符拆分为两级目录）
// 与 keymgr.BackupPath 保持相同的目录结构。
func computeKeyBackupPath(keyFile string) string {
	// 展开路径
	expanded, err := config.ExpandPath(config.KeysDir)
	if err != nil {
		return ""
	}

	// 检查目录是否存在
	if !config.FileExists(expanded) {
		return ""
	}

	// 计算 SHA256 哈希
	hash := sha256Hash(keyFile)

	// 构建三层嵌套目录结构：ab/cd/{hash[4:]}.pem
	fpDir1 := hash[:2]
	fpDir2 := hash[2:4]
	fpFile := hash[4:]
	backupDir := filepath.Join(expanded, fpDir1, fpDir2)

	if !config.FileExists(backupDir) {
		return ""
	}

	return filepath.Join(backupDir, fpFile+".pem")
}

// sha256Hash 计算字符串的 SHA256 哈希（十六进制格式）。
func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

// getHostKeyCallback 返回 HostKey 校验回调函数。
// 默认使用 known_hosts 文件进行校验，仅当 Options["insecure_skip_hostkey"] 为 true 时跳过。
func (c *Connector) getHostKeyCallback(server *domain.Server) ssh.HostKeyCallback {
	// 检查是否跳过 host key 验证
	if skip, ok := server.Options["insecure_skip_hostkey"]; ok {
		if v, ok := skip.(bool); ok && v {
			return ssh.InsecureIgnoreHostKey()
		}
	}

	// 默认：使用 known_hosts 文件验证
	knownHostsPath := c.getKnownHostsPath()
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		// 如果 known_hosts 文件无法加载（不存在或格式错误），
		// 返回自定义回调，提示用户首次连接
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return c.handleUnknownHost(hostname, key, knownHostsPath)
		}
	}

	// 包装 knownhosts callback，未知主机时提示用户
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := callback(hostname, remote, key)
		if err != nil {
			var keyErr *knownhosts.KeyError
			if errors.As(err, &keyErr) {
				// 未知主机，提示用户
				return c.handleUnknownHost(hostname, key, knownHostsPath)
			}
			// 其他错误（如已知主机但密钥不匹配）
			return err
		}
		return nil
	}
}

// getKnownHostsPath 返回 known_hosts 文件路径。
func (c *Connector) getKnownHostsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".ssh", "known_hosts")
}

// handleUnknownHost 处理未知主机的情况。
// 在终端环境中交互式提示用户，非终端环境拒绝连接。
func (c *Connector) handleUnknownHost(hostname string, key ssh.PublicKey, knownHostsPath string) error {
	fingerprint := ssh.FingerprintSHA256(key)

	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Printf("The authenticity of host '%s' can't be established.\n", hostname)
		fmt.Printf("%s key fingerprint is %s\n", key.Type(), fingerprint)
		fmt.Printf("Are you sure you want to continue connecting (yes/no/[fingerprint])? ")

		var response string
		fmt.Scanln(&response)

		switch strings.ToLower(response) {
		case "yes", "y":
			return c.addToKnownHosts(hostname, key, knownHostsPath)
		case fingerprint:
			return c.addToKnownHosts(hostname, key, knownHostsPath)
		default:
			return fmt.Errorf("host key verification failed: user rejected")
		}
	}

	// 非交互式环境：拒绝连接
	return fmt.Errorf("host key verification failed: unknown host %s (fingerprint: %s)", hostname, fingerprint)
}

// addToKnownHosts 将主机密钥添加到 known_hosts 文件。
func (c *Connector) addToKnownHosts(hostname string, key ssh.PublicKey, knownHostsPath string) error {
	dir := filepath.Dir(knownHostsPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create known_hosts directory: %w", err)
	}

	f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open known_hosts: %w", err)
	}
	defer f.Close()

	line := knownhosts.Line([]string{hostname}, key)
	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("failed to write known_hosts: %w", err)
	}

	return nil
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
