package keymgr

import (
	"bytes"
	"crypto/sha256"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"assh/asshc/infra/crypto"
	"assh/config"
	"golang.org/x/crypto/ssh"
)

// marshalAuthorizedKey 将 ssh.PublicKey 编码为 authorized_keys 格式。
// 去掉 ssh.MarshalAuthorizedKey 附加的尾部换行符。
func marshalAuthorizedKey(pubKey ssh.PublicKey) []byte {
	return bytes.TrimRight(ssh.MarshalAuthorizedKey(pubKey), "\n\r")
}

// validKeyTypes 记录支持的密钥类型。
var validKeyTypes = map[string]string{
	"rsa":     "RSA",
	"ed25519": "Ed25519",
	"ecdsa":   "ECDSA",
}

// Generate 生成指定类型的 SSH 密钥对并保存到 data/keys/ 目录。
//
// 参数：
//   - keyType: "rsa"、"ed25519" 或 "ecdsa"
//   - bits: RSA(≥2048, 默认4096) / ECDSA(256/384/521, 默认384) / Ed25519(忽略)
//   - passphrase: 加密私钥的密码（[]byte，便于使用后清零）
//   - comment: 密钥注释（ssh.MarshalPrivateKey 的 comment 参数）
//
// 返回：
//   - privPath: 私钥文件路径 data/keys/{fingerprint}.pem
//   - pubKey:   OpenSSH authorized_keys 格式公钥
//   - fingerprint: SHA256 内容指纹（用于文件名）
func (m *KeyManager) Generate(keyType string, bits int, passphrase []byte, comment string) (privPath string, pubKey []byte, fingerprint string, err error) {
	// 验证密钥类型
	if _, ok := validKeyTypes[keyType]; !ok {
		return "", nil, "", fmt.Errorf("unsupported key type: %q (supported: rsa, ed25519, ecdsa)", keyType)
	}

	// 验证并设置默认 bits
	bits, err = validateBits(keyType, bits)
	if err != nil {
		return "", nil, "", err
	}

	// 填充默认 comment
	if comment == "" {
		comment = "user@host"
	}

	// 生成密钥对
	var privPEM, pubSSH []byte
	switch keyType {
	case "rsa":
		privPEM, pubSSH, err = generateRSA(bits, comment, passphrase)
	case "ed25519":
		privPEM, pubSSH, err = generateEd25519(comment, passphrase)
	case "ecdsa":
		privPEM, pubSSH, err = generateECDSA(bits, comment, passphrase)
	}
	if err != nil {
		return "", nil, "", fmt.Errorf("generate %s key: %w", keyType, err)
	}

	// 计算指纹：SHA256(PEM 内容)
	fp := fmt.Sprintf("%x", sha256.Sum256(privPEM))

	// 构建三层嵌套目录结构：ab/cd/efghijklmnop.pem
	// 前4个字符拆分为两级目录，剩余为文件名
	fpDir1 := fp[:2]
	fpDir2 := fp[2:4]
	fpFile := fp[4:]
	privDir := filepath.Join(m.keysDir, fpDir1, fpDir2)
	privPath = filepath.Join(privDir, fpFile+".pem")

	// 确保 keys 目录和子目录存在
	if err := config.EnsureDir(privDir); err != nil {
		return "", nil, "", fmt.Errorf("ensure keys dir: %w", err)
	}

	// 写入私钥文件（0600 权限）
	if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
		return "", nil, "", fmt.Errorf("write private key: %w", err)
	}

	// 写入公钥文件（0644 权限），后缀为 .pub（不是 .pem.pub）
	pubPath := privDir + "/" + fpFile + ".pub"
	pubContent := append(pubSSH, '\n')
	if err := os.WriteFile(pubPath, pubContent, 0644); err != nil {
		return "", nil, "", fmt.Errorf("write public key: %w", err)
	}

	return privPath, pubSSH, fp, nil
}

// validateBits 验证并标准化密钥位数参数。
// 返回处理后的 bits 值（含默认值填充）。
func validateBits(keyType string, bits int) (int, error) {
	switch keyType {
	case "rsa":
		if bits <= 0 {
			return 4096, nil
		}
		if bits < 2048 {
			return 0, fmt.Errorf("RSA key size must be at least 2048, got %d", bits)
		}
		return bits, nil

	case "ecdsa":
		if bits <= 0 {
			return 384, nil
		}
		switch bits {
		case 256, 384, 521:
			return bits, nil
		default:
			return 0, fmt.Errorf("ECDSA key size must be 256, 384, or 521, got %d", bits)
		}

	case "ed25519":
		return 0, nil // bits 忽略

	default:
		return 0, fmt.Errorf("unsupported key type: %q", keyType)
	}
}

// generateRSA 生成 RSA 密钥对并返回 PEM 私钥和 OpenSSH 公钥。
func generateRSA(bits int, comment string, passphrase []byte) (privPEM, pubSSH []byte, err error) {
	key, err := crypto.GenerateRSA(bits)
	if err != nil {
		return nil, nil, err
	}

	sshPub, err := ssh.NewPublicKey(key.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("new public key: %w", err)
	}
	pubSSH = marshalAuthorizedKey(sshPub)

	privPEM, err = marshalPrivateKey(key.PrivateKey, comment, passphrase)
	if err != nil {
		return nil, nil, err
	}

	return privPEM, pubSSH, nil
}

// generateEd25519 生成 Ed25519 密钥对并返回 PEM 私钥和 OpenSSH 公钥。
func generateEd25519(comment string, passphrase []byte) (privPEM, pubSSH []byte, err error) {
	key, err := crypto.GenerateEd25519()
	if err != nil {
		return nil, nil, err
	}

	sshPub, err := ssh.NewPublicKey(key.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("new public key: %w", err)
	}
	pubSSH = marshalAuthorizedKey(sshPub)

	privPEM, err = marshalPrivateKey(key.PrivateKey, comment, passphrase)
	if err != nil {
		return nil, nil, err
	}

	return privPEM, pubSSH, nil
}

// generateECDSA 生成 ECDSA 密钥对并返回 PEM 私钥和 OpenSSH 公钥。
func generateECDSA(bits int, comment string, passphrase []byte) (privPEM, pubSSH []byte, err error) {
	var ecdsaKey *crypto.ECDSAKey
	switch bits {
	case 256:
		ecdsaKey, err = crypto.GenerateP256ECDSA()
	case 384:
		ecdsaKey, err = crypto.GenerateP384ECDSA()
	case 521:
		ecdsaKey, err = crypto.GenerateP521ECDSA()
	default:
		return nil, nil, fmt.Errorf("unsupported ECDSA bits: %d", bits)
	}
	if err != nil {
		return nil, nil, err
	}

	sshPub, err := ssh.NewPublicKey(ecdsaKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("new public key: %w", err)
	}
	pubSSH = marshalAuthorizedKey(sshPub)

	privPEM, err = marshalPrivateKey(ecdsaKey.PrivateKey, comment, passphrase)
	if err != nil {
		return nil, nil, err
	}

	return privPEM, pubSSH, nil
}

// keyTypeDefaultFilenames 映射密钥类型到 ssh-keygen 兼容的默认文件名。
var keyTypeDefaultFilenames = map[string]string{
	"rsa":     "id_rsa",
	"ed25519": "id_ed25519",
	"ecdsa":   "id_ecdsa",
}

// GenerateToPath 生成 SSH 密钥对并保存到指定路径（生成模式）。
//
// 与 Generate() 不同，此方法不保存到 data/keys/{fingerprint}.pem，
// 而是直接将私钥写到 outputPath，公钥写到 outputPath + ".pub"。
//
// 路径处理规则：
//   - outputPath 是目录（以 / 结尾或已存在目录）→ 自动追加默认文件名
//   - outputPath 是文件路径 → 直接使用（父目录自动创建）
//   - 文件已存在时直接覆盖
//
// 返回公钥内容（OpenSSH authorized_keys 格式）和私钥 SHA256 指纹。
func (m *KeyManager) GenerateToPath(keyType string, bits int, passphrase []byte, comment string, outputPath string) (pubKey []byte, fingerprint string, err error) {
	// 1. 验证密钥类型
	if _, ok := validKeyTypes[keyType]; !ok {
		return nil, "", fmt.Errorf("unsupported key type: %q (supported: rsa, ed25519, ecdsa)", keyType)
	}

	// 2. 验证并标准化 bits
	bits, err = validateBits(keyType, bits)
	if err != nil {
		return nil, "", err
	}

	// 3. 填充默认 comment
	if comment == "" {
		comment = "user@host"
	}

	// 4. 解析输出路径（目录 → 追加默认文件名）
	resolvedPath, err := resolveKeyOutputPath(outputPath, keyType)
	if err != nil {
		return nil, "", fmt.Errorf("resolve output path: %w", err)
	}

	// 5. 生成密钥对
	var privPEM, pubSSH []byte
	switch keyType {
	case "rsa":
		privPEM, pubSSH, err = generateRSA(bits, comment, passphrase)
	case "ed25519":
		privPEM, pubSSH, err = generateEd25519(comment, passphrase)
	case "ecdsa":
		privPEM, pubSSH, err = generateECDSA(bits, comment, passphrase)
	}
	if err != nil {
		return nil, "", fmt.Errorf("generate %s key: %w", keyType, err)
	}

	// 6. 计算指纹
	fp := fmt.Sprintf("%x", sha256.Sum256(privPEM))

	// 7. 确保父目录存在
	if err := config.EnsureDir(filepath.Dir(resolvedPath)); err != nil {
		return nil, "", fmt.Errorf("ensure directory: %w", err)
	}

	// 8. 写入私钥（0600）
	if err := os.WriteFile(resolvedPath, privPEM, 0600); err != nil {
		return nil, "", fmt.Errorf("write private key: %w", err)
	}

	// 9. 写入公钥（0644），带尾部换行符（ssh-keygen 兼容格式）
	pubPath := resolvedPath + ".pub"
	pubContent := append(pubSSH, '\n')
	if err := os.WriteFile(pubPath, pubContent, 0644); err != nil {
		return nil, "", fmt.Errorf("write public key: %w", err)
	}

	return pubSSH, fp, nil
}

// resolveKeyOutputPath 解析 --output 参数为最终文件路径。
// 如果 outputPath 是目录则追加默认密钥文件名，否则直接使用。
// 使用 config.ResolveKeyOutputPath 公共函数。
func resolveKeyOutputPath(outputPath, keyType string) (string, error) {
	return config.ResolveKeyOutputPath(outputPath, keyType)
}

// GenerateToExistingPath 生成 SSH 密钥对并保存到指定路径（覆盖已有文件）。
// 与 Generate() 不同，此方法复用指定路径而非按指纹命名。
// 路径格式支持嵌套目录（如 ab/cd/efg.pem）。
//
// 参数：
//   - keyType: "rsa"、"ed25519" 或 "ecdsa"
//   - bits: RSA(≥2048, 默认4096) / ECDSA(256/384/521, 默认384) / Ed25519(忽略)
//   - passphrase: 加密私钥的密码
//   - comment: 密钥注释
//   - privPath: 已有私钥路径（将覆盖此文件）
//
// 返回：
//   - pubKey: OpenSSH authorized_keys 格式公钥
//   - err: 错误信息
func (m *KeyManager) GenerateToExistingPath(keyType string, bits int, passphrase []byte, comment string, privPath string) (pubKey []byte, err error) {
	// 验证密钥类型
	if _, ok := validKeyTypes[keyType]; !ok {
		return nil, fmt.Errorf("unsupported key type: %q", keyType)
	}

	// 验证并设置默认 bits
	bits, err = validateBits(keyType, bits)
	if err != nil {
		return nil, err
	}

	// 填充默认 comment
	if comment == "" {
		comment = "user@host"
	}

	// 生成密钥对
	var privPEM []byte
	switch keyType {
	case "rsa":
		privPEM, pubKey, err = generateRSA(bits, comment, passphrase)
	case "ed25519":
		privPEM, pubKey, err = generateEd25519(comment, passphrase)
	case "ecdsa":
		privPEM, pubKey, err = generateECDSA(bits, comment, passphrase)
	}
	if err != nil {
		return nil, fmt.Errorf("generate %s key: %w", keyType, err)
	}

	// 确保父目录存在
	if err := config.EnsureDir(filepath.Dir(privPath)); err != nil {
		return nil, fmt.Errorf("ensure directory: %w", err)
	}

	// 写入私钥文件（0600 权限，覆盖已有文件）
	if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
		return nil, fmt.Errorf("write private key: %w", err)
	}

	// 写入公钥文件（0644 权限），后缀为 .pub（不是 .pem.pub）
	pubPath := strings.TrimSuffix(privPath, ".pem") + ".pub"
	pubContent := append(pubKey, '\n')
	if err := os.WriteFile(pubPath, pubContent, 0644); err != nil {
		return nil, fmt.Errorf("write public key: %w", err)
	}

	return pubKey, nil
}

// marshalPrivateKey 将私钥编码为 PEM 格式的 OpenSSH 私钥。
// 如果 passphrase 非空，使用 ssh.MarshalPrivateKeyWithPassphrase 加密；
// 否则使用 ssh.MarshalPrivateKey 生成未加密密钥。
// comment 参数设置密钥注释，会被写入 PEM 头部的 Comment 字段。
func marshalPrivateKey(key interface{}, comment string, passphrase []byte) ([]byte, error) {
	var block *pem.Block
	var err error

	if len(passphrase) > 0 {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(key, comment, passphrase)
	} else {
		block, err = ssh.MarshalPrivateKey(key, comment)
	}
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	if block == nil {
		return nil, errors.New("nil PEM block from marshal")
	}

	privPEM := pem.EncodeToMemory(block)
	if privPEM == nil {
		return nil, errors.New("nil PEM encoding result")
	}
	return privPEM, nil
}
