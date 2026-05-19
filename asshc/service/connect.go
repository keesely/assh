package service

import (
	"fmt"
	"strconv"
	"strings"

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

// ConnectChain 通过指定跳板链连接到目标服务器。
// chainExpr 的格式：逗号分隔的跳板列表
//   每条记录可以是：服务器名 或 @history_id
//      服务器名 → 从 DB 读取配置
//      @id      → 从 jump_history 读取并恢复跳板链
//      user@host[:port] → 直连参数构建服务器对象
func (s *ConnectService) ConnectChain(target string, chainExpr string) (*ssh.Client, []*domain.Server, error) {
	// 解析目标服务器
	targetServer, err := s.resolveTarget(target)
	if err != nil {
		return nil, nil, err
	}

	// 解析跳板链
	chain, err := s.parseJumpChain(chainExpr)
	if err != nil {
		return nil, nil, err
	}

	// 通过跳板链连接
	client, err := s.connector.ConnectChain(targetServer, chain)
	if err != nil {
		return nil, nil, err
	}
	return client, chain, nil
}

// resolveTarget 解析目标服务器（已保存名称 或 user@host 格式）
func (s *ConnectService) resolveTarget(target string) (*domain.Server, error) {
	if strings.Contains(target, "@") {
		// 直连参数
		parts := strings.SplitN(target, "@", 2)
		user, host := parts[0], parts[1]
		// 解析 port
		host, portStr, _ := strings.Cut(host, ":")
		port := 22
		if portStr != "" {
			port, _ = strconv.Atoi(portStr)
		}
		if user == "" {
			user = "root"
		}
		return &domain.Server{
			Host: host,
			Port: port,
			User: user,
		}, nil
	}

	// 从 DB 读取
	return s.repo.Get(target)
}

// parseJumpChain 解析跳板链表达式为 *domain.Server 列表
// 返回值还包括是否包含 direct 类型（需要记录历史）
func (s *ConnectService) parseJumpChain(chainExpr string) ([]*domain.Server, error) {
	if chainExpr == "" || chainExpr == "none" {
		return nil, nil
	}

	tokens := strings.Split(chainExpr, ",")
	var chain []*domain.Server

	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		switch {
		case strings.HasPrefix(token, "@"):
			// history ref: @id
			// TODO: 从 jump_history 读取并恢复，实现 P10.7 时完成
		case strings.Contains(token, "@"):
			// direct: user@host[:port]
			parts := strings.SplitN(token, "@", 2)
			user, host := parts[0], parts[1]
			host, portStr, _ := strings.Cut(host, ":")
			port := 22
			if portStr != "" {
				port, _ = strconv.Atoi(portStr)
			}
			if user == "" {
				user = "root"
			}
			chain = append(chain, &domain.Server{
				Host: host,
				Port: port,
				User: user,
			})
		default:
			// server ref: 从 DB 读取
			server, err := s.repo.Get(token)
			if err != nil {
				return nil, fmt.Errorf("resolve jump host %q: %w", token, err)
			}
			chain = append(chain, server)
		}
	}

	return chain, nil
}
