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
	// 使用 passphrase 加密私钥（ssh.MarshalPrivateKey）。
	// 私钥保存到 data/keys/{fingerprint}.pem，返回私钥路径、公钥（OpenSSH authorized_keys 格式）和指纹。
	//
	// 验证契约：
	// - keyType 必须为 "rsa"、"ed25519" 或 "ecdsa"，不支持的类型返回错误
	// - RSA：bits 必须 >= 2048（小于 2048 不安全），默认 4096，不合法 bits 返回错误
	// - ECDSA：bits 必须为 256、384 或 521，默认 384，不合法 bits 返回错误
	// - Ed25519：忽略 bits 参数，固定生成标准 Ed25519 密钥
	// - passphrase 传入 []byte 便于使用后清零（不可为 nil，否则按空 passphrase 处理）
	Generate(keyType string, bits int, passphrase []byte) (privPath string, pubKey []byte, fingerprint string, err error)

	// DeployPublicKey SSH 登录服务器，将公钥追加到 ~/.ssh/authorized_keys。
	// 幂等实现：解析 authorized_keys 的公钥指纹，如果目标公钥已存在则跳过。
	// 如果 authorized_keys 不存在，自动创建（mkdir -p ~/.ssh + touch）。
	DeployPublicKey(server *domain.Server, pubKey []byte) error

	// BackupKey 将指定路径的密钥文件复制到 data/keys/ 目录。
	// 私钥文件权限必须为 0600 或更严格（文件权限掩码 <= 0x077）。
	// 如果文件权限过宽（>0600），返回错误。
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
	// 返回 []byte 便于使用后清零。如果未设置 account password，返回 nil（兼容模式）。
	// 调用者在使用后应将返回的 []byte 填充为零值以防内存残留。
	GetAccountPassphrase() []byte
}
