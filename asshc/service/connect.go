package service

import (
	"fmt"

	"assh/asshc/domain"
	"assh/asshc/port"
	"golang.org/x/crypto/ssh"
)

// ConnectService 封装 SSH 连接和会话管理的业务逻辑。
// 支持通过已保存的服务器名称连接和直接指定连接参数两种方式。
// 依赖 SSHConnector（建立连接）和 SSHSession（执行命令）两个接口。
type ConnectService struct {
	connector port.SSHConnector
	session   port.SSHSession
	repo      port.ServerRepository
}

// NewConnectService 创建 ConnectService 实例，注入所有依赖。
func NewConnectService(
	connector port.SSHConnector,
	session port.SSHSession,
	repo port.ServerRepository,
) *ConnectService {
	return &ConnectService{
		connector: connector,
		session:   session,
		repo:      repo,
	}
}

// ConnectByName 根据已保存的服务器名称建立 SSH 连接。
// 从存储中查询服务器配置，然后使用配置信息建立连接。
func (s *ConnectService) ConnectByName(name string) (*ssh.Client, error) {
	if name == "" {
		return nil, domain.ErrInvalidName
	}

	server, err := s.repo.Get(name)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, domain.ErrNotFound
	}

	return s.connector.Connect(server)
}

// ConnectDirect 使用直接指定的参数建立 SSH 连接，不依赖已保存的配置。
// 支持 host、port、user、password、keyFile 等参数。
// 如果 keyBackupPath 不为空，优先使用该路径的密钥文件（用于直连 keygen 后复用密钥）。
func (s *ConnectService) ConnectDirect(host string, port int, user, password, keyFile string, keyBackupPath string) (*ssh.Client, error) {
	if host == "" {
		return nil, fmt.Errorf("host is required")
	}

	server := &domain.Server{
		Host: host,
		Port: port,
		User: user,
		Auth: &domain.Auth{
			Password: password,
			KeyFile:  keyFile,
		},
	}

	// P6.6-b: 如果有 known_servers 中的 key_backup_path，优先使用
	if keyBackupPath != "" {
		server.Auth.KeyFile = keyBackupPath
	}

	if server.Port <= 0 {
		server.Port = 22
	}
	if server.User == "" {
		server.User = "root"
	}

	return s.connector.Connect(server)
}

// Shell 在已建立的 SSH 连接上启动交互式 Shell 会话。
func (s *ConnectService) Shell(client *ssh.Client) error {
	if client == nil {
		return fmt.Errorf("ssh client is nil")
	}
	return s.session.Shell(client)
}

// Run 在已建立的 SSH 连接上执行命令，输出写入标准输出。
func (s *ConnectService) Run(client *ssh.Client, cmd string) error {
	if client == nil {
		return fmt.Errorf("ssh client is nil")
	}
	if cmd == "" {
		return fmt.Errorf("command is empty")
	}
	return s.session.Run(client, cmd)
}

// RunWithOutput 在已建立的 SSH 连接上执行命令，将完整输出作为字符串返回。
func (s *ConnectService) RunWithOutput(client *ssh.Client, cmd string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("ssh client is nil")
	}
	if cmd == "" {
		return "", fmt.Errorf("command is empty")
	}
	return s.session.RunWithOutput(client, cmd)
}

// Close 安全关闭 SSH 连接，如果连接已为 nil 则不做操作。
func (s *ConnectService) Close(client *ssh.Client) error {
	if client == nil {
		return nil
	}
	return s.connector.Close(client)
}

// Connector 返回底层的 SSHConnector，用于 DeployService 等场景。
func (s *ConnectService) Connector() port.SSHConnector {
	return s.connector
}
