// Package port 定义核心业务接口（端口），实现依赖倒置。
//
// 本包只定义接口和领域类型引用，不包含任何实现。
// service 层依赖本包的接口，infra 层提供具体实现，
// 从而实现依赖倒置原则——高层模块不依赖低层模块的具体实现。
package port

import "assh/asshc/domain"

// KeyManager 定义 SSH 密钥管理的核心接口。
// 提供密钥生成、部署、备份、查找等功能，用于实现 Phase 6 密钥管理模块。
// 私钥使用 account password 加密后存储到 data/keys/ 目录，登录时解密使用。
type KeyManager interface {
	// Generate 生成指定类型的 SSH 密钥对。
	// 使用 account password 作为 passphrase 加密私钥（ssh.MarshalPrivateKey）。
	// 私钥保存到 data/keys/{fingerprint}.pem，返回私钥路径、公钥（OpenSSH authorized_keys 格式）和指纹。
	// keyType 支持 "rsa"/"ed25519"/"ecdsa"，bits 对 RSA/ECDSA 有效（RSA 默认 4096，ECDSA 默认 384）。
	Generate(keyType string, bits int, passphrase string) (privPath string, pubKey []byte, fingerprint string, err error)

	// DeployPublicKey SSH 登录服务器，将公钥追加到 ~/.ssh/authorized_keys。
	// 幂等实现：解析 authorized_keys 的公钥指纹，如果目标公钥已存在则跳过。
	// 如果 authorized_keys 不存在，自动创建（mkdir -p ~/.ssh + touch）。
	DeployPublicKey(server *domain.Server, pubKey []byte) error

	// BackupKey 将指定路径的密钥文件复制到 data/keys/ 目录。
	// 私钥文件权限必须为 0600 或更严格。
	// 根据私钥内容计算 SHA256 指纹作为文件名，返回备份路径和指纹。
	// 如果文件已存在于 data/keys/，返回已存在的路径。
	BackupKey(srcPath string) (backupPath string, fingerprint string, err error)

	// LookupByPath 在 data/keys/ 中查找与原始路径匹配的备份密钥。
	// 用于 SSH 连接时优先使用备份密钥（防止原始路径被删除或篡改）。
	// 通过 data/keys/ 目录中的索引文件或文件名匹配实现。
	LookupByPath(originalPath string) (backupPath string, found bool)

	// ReadKeyFromStdin 从标准输入读取密钥内容（OpenSSH 格式 PEM），保存到 data/keys/ 目录。
	// 返回保存后的路径和指纹。
	// stdin 非 tty 时触发，支持管道和重定向（cat key.pem | assh set myserver -k）。
	ReadKeyFromStdin() (savedPath string, fingerprint string, err error)

	// GetAccountPassphrase 获取 account password，用于解密私钥。
	// 如果未设置 account password，返回空字符串（兼容模式）。
	GetAccountPassphrase() string
}
