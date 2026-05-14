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
	// - comment 设置密钥注释（ssh.MarshalPrivateKey 的 comment 参数），默认 "user@host"
	Generate(keyType string, bits int, passphrase []byte, comment string) (privPath string, pubKey []byte, fingerprint string, err error)

	// DeployPublicKey SSH 登录服务器，将公钥追加到 ~/.ssh/authorized_keys。
	// 幂等实现：解析 authorized_keys 的公钥指纹，如果目标公钥已存在则跳过。
	// 如果 authorized_keys 不存在，自动创建（mkdir -p ~/.ssh + touch）。
	DeployPublicKey(server *domain.Server, pubKey []byte) error

	// BackupKey 将指定路径的密钥文件备份到 destPath（快照模式）。
	// 私钥文件权限必须为 0600 或更严格（无 group/other 访问权限）。
	// 如果权限过宽（group/other 可读），返回错误。
	// 根据源文件内容计算 SHA256 指纹并返回。
	// destPath 已存在时直接覆盖（始终反映最新快照）。
	BackupKey(srcPath string, destPath string) (fingerprint string, err error)

	// BackupPath 计算指定服务器名的备份路径并检查文件是否存在。
	// name 为服务器标识（如 group.name），用于生成哈希文件名。
	// 返回完整的备份路径和是否存在。
	// 用于 SSH 连接时优先使用备份快照（data/keys/{sha256(name)}.pem）。
	BackupPath(name string) (path string, exists bool)

	// ReadKeyFromStdin 从标准输入读取密钥内容（OpenSSH 格式 PEM），保存到 data/keys/ 目录。
	// 返回保存后的路径和指纹。
	// stdin 非 tty 时触发，支持管道和重定向（cat key.pem | assh set myserver -k）。
	ReadKeyFromStdin() (savedPath string, fingerprint string, err error)

	// GenerateToPath 生成指定类型的 SSH 密钥对并保存到指定路径。
	// 私钥保存到 outputPath，公钥保存到 outputPath + ".pub"。
	// 与 Generate 不同，此方法不保存到 data/keys/{fingerprint}.pem，
	// 而是直接写入调用者指定的路径。
	//
	// 路径处理规则：
	//   - outputPath 以路径分隔符结尾（如 ~/.ssh/）或指向已存在的目录时，
	//     自动追加默认密钥文件名（id_rsa / id_ed25519 / id_ecdsa）
	//   - outputPath 为文件路径时直接使用
	//   - 父目录不存在时自动创建
	//   - 文件已存在时直接覆盖
	//
	// 验证契约同 Generate：keyType/bits/comment 规则一致。
	//
	// 返回公钥内容（OpenSSH authorized_keys 格式）和私钥 SHA256 指纹。
	GenerateToPath(keyType string, bits int, passphrase []byte, comment string, outputPath string) (pubKey []byte, fingerprint string, err error)

	// GenerateToExistingPath 生成 SSH 密钥对并保存到指定路径（覆盖已有文件）。
	// 与 GenerateToPath 不同，此方法复用指定路径而非按指纹命名。
	// 路径格式支持嵌套目录（如 ab/cd/efg.pem）。
	// 文件已存在时直接覆盖。
	//
	// 验证契约同 Generate：keyType/bits/comment 规则一致。
	//
	// 返回公钥内容（OpenSSH authorized_keys 格式）。
	GenerateToExistingPath(keyType string, bits int, passphrase []byte, comment string, privPath string) (pubKey []byte, err error)

	// GetAccountPassphrase 获取 account password，用于解密私钥。
	// 返回 []byte 便于使用后清零。如果未设置 account password，返回 nil（兼容模式）。
	// 调用者在使用后应将返回的 []byte 填充为零值以防内存残留。
	GetAccountPassphrase() []byte
}
