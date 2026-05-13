// Package service 实现业务用例编排层。
//
// 本服务层负责组合 port 接口来完成具体业务逻辑。
// 依赖的接口通过构造函数注入，不直接依赖具体的 infra 实现。
// 主要包含 KeyService（密钥管理服务）、ServerService（服务器配置管理）
// 和 ConnectService（SSH 连接管理）。
package service

import (
	"bytes"
	"fmt"
	"strings"

	"assh/asshc/domain"
	"assh/asshc/port"
	"golang.org/x/crypto/ssh"
)

// DeployService 封装 SSH 公钥部署的业务逻辑。
// 提供将公钥追加到远程服务器 ~/.ssh/authorized_keys 的功能。
//
// 幂等策略：解析 authorized_keys 的公钥指纹，如果目标公钥已存在则跳过。
// authorized_keys 不存在时自动创建（mkdir -p ~/.ssh + touch）。
type DeployService struct {
	connector port.SSHConnector
}

// NewDeployService 创建 DeployService 实例，注入 SSH 连接器。
func NewDeployService(connector port.SSHConnector) *DeployService {
	return &DeployService{
		connector: connector,
	}
}

// DeployToServer 将公钥部署到指定服务器的 ~/.ssh/authorized_keys。
//
// 幂等流程：
//  1. SSH 登录服务器
//  2. 读取 ~/.ssh/authorized_keys（不存在则创建）
//  3. 解析每行公钥指纹
//  4. 指纹已存在 → 跳过（幂等）
//  5. 指纹不存在 → 追加公钥
//
// 参数：
//   - server:  服务器配置（包含 Host/Port/User/Auth）
//   - pubKey:  OpenSSH authorized_keys 格式的公钥内容
//
// 返回：
//   - deployed: 是否成功追加了公钥（false 表示已存在，无需追加）
//   - err:      操作过程中的错误
func (s *DeployService) DeployToServer(server *domain.Server, pubKey []byte) (deployed bool, err error) {
	if server == nil {
		return false, fmt.Errorf("server is nil")
	}
	if len(pubKey) == 0 {
		return false, fmt.Errorf("public key is empty")
	}

	// 1. SSH 登录服务器
	client, err := s.connector.Connect(server)
	if err != nil {
		return false, fmt.Errorf("SSH connect: %w", err)
	}
	defer s.connector.Close(client)

	// 2. 创建 SSH 会话读取 authorized_keys
	session, err := client.NewSession()
	if err != nil {
		return false, fmt.Errorf("create session: %w", err)
	}

	// 3. 读取 authorized_keys 内容
	authorizedKeys, err := s.readAuthorizedKeys(session)
	session.Close() // 读取后关闭 session
	if err != nil {
		return false, fmt.Errorf("read authorized_keys: %w", err)
	}

	// 4. 计算公钥指纹
	pubKeyFingerprint := normalizePubKey(pubKey)

	// 5. 检查是否已存在（幂等）
	if containsFingerprint(authorizedKeys, pubKeyFingerprint) {
		return false, nil // 已存在，无需追加
	}

	// 6. 创建新的 SSH 会话追加公钥（每个 session.Run() 只能执行一次命令）
	session2, err := client.NewSession()
	if err != nil {
		return false, fmt.Errorf("create session for append: %w", err)
	}
	defer session2.Close()

	// 追加公钥到 authorized_keys
	if err := s.appendToAuthorizedKeys(session2, pubKey); err != nil {
		return false, fmt.Errorf("append to authorized_keys: %w", err)
	}

	return true, nil
}

// readAuthorizedKeys 读取远程服务器的 ~/.ssh/authorized_keys 内容。
// 如果文件不存在，返回空字符串。
func (s *DeployService) readAuthorizedKeys(session *ssh.Session) (string, error) {
	// 读取 authorized_keys 内容（忽略不存在的错误）
	var stdout bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &bytes.Buffer{}

	// 使用 cat 读取文件，不存在时输出为空
	if err := session.Run("cat ~/.ssh/authorized_keys 2>/dev/null || true"); err != nil {
		return "", err
	}

	return stdout.String(), nil
}

// appendToAuthorizedKeys 将公钥追加到远程服务器的 ~/.ssh/authorized_keys。
// 如果 ~/.ssh 目录不存在，自动创建。
// 使用单次 session.Run() 执行所有操作，避免 "session already started" 错误。
func (s *DeployService) appendToAuthorizedKeys(session *ssh.Session, pubKey []byte) error {
	// 构建命令，转义单引号
	pubKeyStr := strings.ReplaceAll(string(pubKey), "'", "'\"'\"'")

	// 使用单次 SSH session 执行所有操作：
	// 1. 创建 ~/.ssh 目录并设置权限
	// 2. 追加公钥到 authorized_keys
	// 3. 设置 authorized_keys 权限
	cmd := fmt.Sprintf(
		`mkdir -p ~/.ssh && chmod 700 ~/.ssh && printf '%%s\n' '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`,
		pubKeyStr,
	)

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("deploy key: %w", err)
	}

	return nil
}

// normalizePubKey 规范化公钥内容（去除首尾空白），用于比较和去重。
func normalizePubKey(pubKey []byte) string {
	return strings.TrimSpace(string(pubKey))
}

// containsFingerprint 检查 authorized_keys 内容中是否包含指定的公钥。
func containsFingerprint(authorizedKeys, pubKey string) bool {
	lines := strings.Split(authorizedKeys, "\n")
	pubKeyTrimmed := strings.TrimSpace(pubKey)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 直接比较公钥内容（去除末尾空白）
		if line == pubKeyTrimmed {
			return true
		}
	}

	return false
}
