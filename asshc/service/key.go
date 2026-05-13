// Package service 实现业务用例编排层。
//
// 本服务层负责组合 port 接口来完成具体业务逻辑。
// 依赖的接口通过构造函数注入，不直接依赖具体的 infra 实现。
// 主要包含 KeyService（密钥管理服务）、ServerService（服务器配置管理）
// 和 ConnectService（SSH 连接管理）。
package service

import (
	"fmt"
	"os"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// KeyService 封装 SSH 密钥管理的业务逻辑。
// 提供密钥生成部署（GenerateAndDeploy）和 -k 参数处理（HandleKeyFlag）两大用例。
//
// 三个操作的业务流程：
//
//	GenerateAndDeploy:  生成密钥 → 部署公钥到服务器 → 更新配置
//	HandleKeyFlag(""):  调用 GenerateAndDeploy（默认 RSA 4096）
//	HandleKeyFlag("-"): 从 stdin 读取密钥内容 → 保存 → 更新配置
//	HandleKeyFlag(path):备份密钥文件 → 更新配置
type KeyService struct {
	keymgr   port.KeyManager
	repo     port.ServerRepository
	ssh      port.SSHConnector  // SSH 连接器（用于公钥部署）
	deployer *DeployService     // 公钥部署服务（P6.2-b）
}

// NewKeyService 创建 KeyService 实例，注入所有依赖。
//
// 参数：
//   - keymgr: 密钥管理器（生成/部署/备份）
//   - repo:   服务器配置仓库（读取/更新服务器配置）
//   - ssh:    SSH 连接器（用于公钥部署）
func NewKeyService(keymgr port.KeyManager, repo port.ServerRepository, ssh port.SSHConnector) *KeyService {
	return &KeyService{
		keymgr:   keymgr,
		repo:     repo,
		ssh:      ssh,
		deployer: NewDeployService(ssh),
	}
}

// GenerateAndDeploy 为指定服务器生成 SSH 密钥对，部署公钥并更新配置。
//
// 完整流程：
//  1. 从 repo 获取服务器配置（确保服务器存在）
//  2. 检查是否有已有密钥路径（Auth.KeyFile 存在），有则复用，无则生成新路径
//  3. 调用 keymgr.GenerateToExistingPath 生成指定类型的密钥对
//  4. 调用 DeployService.DeployToServer 将公钥追加到服务器 ~/.ssh/authorized_keys
//  5. 更新服务器配置 Auth.KeyFile 指向生成的私钥路径
//
// 复用策略：
//   - 如果 server.Auth.KeyFile 已配置且文件存在，则复用该路径生成密钥（覆盖）
//   - 否则生成新路径（嵌套目录结构：ab/cd/{fingerprint}.pem）
//
// 部署容错：
//   - 部署失败不会导致整个操作失败（密钥已生成并配置）
//   - 公钥重复部署被幂等处理（已存在则跳过）
//
// 参数：
//   - name:    服务器完整名称（group.name）
//   - keyType: 密钥类型（"rsa", "ed25519", "ecdsa"）
//   - bits:    密钥位数（RSA: ≥2048, ECDSA: 256/384/521, Ed25519: 忽略）
//   - comment: 密钥注释（默认 "user@host"）
func (s *KeyService) GenerateAndDeploy(name string, keyType string, bits int, comment string) error {
	if name == "" {
		return domain.ErrInvalidName
	}

	// 1. 获取服务器配置
	server, err := s.repo.Get(name)
	if err != nil {
		return fmt.Errorf("get server %q: %w", name, err)
	}
	if server == nil {
		return domain.ErrNotFound
	}

	passphrase := s.keymgr.GetAccountPassphrase()

	// 2. 检查是否有已有密钥路径（复用策略）
	privPath := ""
	if server.Auth != nil && server.Auth.KeyFile != "" {
		if _, statErr := os.Stat(server.Auth.KeyFile); statErr == nil {
			// 文件存在，复用该路径
			privPath = server.Auth.KeyFile
		}
	}

	var pubKey []byte
	if privPath != "" {
		// 复用已有路径：生成密钥到指定路径
		pubKey, err = s.keymgr.GenerateToExistingPath(keyType, bits, passphrase, comment, privPath)
		if err != nil {
			return fmt.Errorf("generate key: %w", err)
		}
	} else {
		// 新生成路径：使用 Generate 获取嵌套目录路径
		privPath, pubKey, _, err = s.keymgr.Generate(keyType, bits, passphrase, comment)
		if err != nil {
			return fmt.Errorf("generate key: %w", err)
		}
	}

	// 3. 部署公钥到服务器（P6.2-b）
	deployed, deployErr := s.deployer.DeployToServer(server, pubKey)
	if deployErr != nil {
		// 部署失败不影响主流程，记录警告
		fmt.Fprintf(os.Stderr, "Warning: failed to deploy public key: %v\n", deployErr)
	} else if deployed {
		fmt.Fprintf(os.Stderr, "Info: public key deployed to %s\n", name)
	} else {
		// 公钥已存在（幂等）
		fmt.Fprintf(os.Stderr, "Info: public key already exists on %s (skipped)\n", name)
	}

	// 4. 更新服务器配置 Auth.KeyFile
	if server.Auth == nil {
		server.Auth = &domain.Auth{}
	}
	server.Auth.KeyFile = privPath

	if err := s.repo.Set(name, server); err != nil {
		_ = os.Remove(privPath) // 配置更新失败 → 清理私钥
		return fmt.Errorf("update server config: %w", err)
	}

	return nil
}

// HandleKeyFlag 处理 -k 参数的三种使用场景，更新服务器配置并返回密钥路径。
//
// 场景一（keyValue == ""）：
//   - 等效于调用 GenerateAndDeploy(name, "rsa", 4096)
//   - 生成 RSA 4096 密钥 → 部署 → 更新配置
//
// 场景二（keyValue == "-"）：
//   - 从标准输入读取密钥内容（OpenSSH PEM 格式）
//   - 保存到 data/keys/{fingerprint}.pem
//   - 更新服务器配置 Auth.KeyFile
//
// 场景三（keyValue 为文件路径）：
//   - 验证文件存在
//   - 计算服务器备份路径 data/keys/{sha256(name)}.pem
//   - 将密钥文件快照到备份路径
//   - 更新服务器配置 Auth.KeyFile
//
// 返回值：
//   - keyFilePath: 更新后的密钥文件路径（用于显示或后续操作）
//   - err:         处理过程中的错误
func (s *KeyService) HandleKeyFlag(name, keyValue string) (keyFilePath string, err error) {
	if name == "" {
		return "", domain.ErrInvalidName
	}

	// 确保服务器存在
	if _, err := s.repo.Get(name); err != nil {
		return "", fmt.Errorf("get server %q: %w", name, err)
	}

	switch {
	case keyValue == "":
		// 场景一：生成新密钥
		if err := s.GenerateAndDeploy(name, "rsa", 4096, ""); err != nil {
			return "", err
		}
		// 返回更新后的密钥路径
		server, getErr := s.repo.Get(name)
		if getErr != nil {
			return "", getErr
		}
		if server.Auth != nil {
			return server.Auth.KeyFile, nil
		}
		return "", nil

	case keyValue == "-":
		// 场景二：从 stdin 读取
		savedPath, _, err := s.keymgr.ReadKeyFromStdin()
		if err != nil {
			return "", fmt.Errorf("read key from stdin: %w", err)
		}
		if err := s.updateKeyFile(name, savedPath); err != nil {
			return "", err
		}
		return savedPath, nil

	default:
		// 场景三：备份已有密钥文件
		// 检查文件是否存在
		if _, statErr := os.Stat(keyValue); statErr != nil {
			return "", fmt.Errorf("key file %q: %w", keyValue, statErr)
		}

		// 计算备份目标路径
		backupPath, _ := s.keymgr.BackupPath(name)

		// 备份密钥文件到目标路径
		if _, backupErr := s.keymgr.BackupKey(keyValue, backupPath); backupErr != nil {
			return "", fmt.Errorf("backup key: %w", backupErr)
		}

		// 更新服务器配置
		if err := s.updateKeyFile(name, backupPath); err != nil {
			return "", err
		}
		return backupPath, nil
	}
}

// updateKeyFile 更新指定服务器的 Auth.KeyFile 配置。
func (s *KeyService) updateKeyFile(name, keyFilePath string) error {
	server, err := s.repo.Get(name)
	if err != nil {
		return fmt.Errorf("get server %q: %w", name, err)
	}
	if server == nil {
		return domain.ErrNotFound
	}
	if server.Auth == nil {
		server.Auth = &domain.Auth{}
	}
	server.Auth.KeyFile = keyFilePath
	return s.repo.Set(name, server)
}
