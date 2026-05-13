// Package keymgr 实现 SSH 密钥管理的基础设施层。
//
// 提供密钥生成（RSA/Ed25519/ECDSA）、密钥备份快照、stdin 密钥读取、
// account passphrase 管理等功能。实现了 port.KeyManager 接口（隐式满足）。
// 密钥文件存储到 ~/.assh/v2/data/keys/ 目录：
//   - 生成的密钥按 SHA256 内容指纹命名：{fingerprint}.pem
//     （keygen 时更新 Auth.KeyFile 指向该路径）
//   - 备份快照按服务器名哈希命名：{sha256(serverName)}.pem
//     （-k 备份时不修改 Auth.KeyFile，快照仅用于登录优先查找）
//   - 登录优先级：快照 → Auth.KeyFile → 密码
package keymgr

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"assh/asshc/domain"
	"assh/config"
)

// KeyManager 实现 SSH 密钥管理功能。
// 提供密钥生成、备份快照、stdin 读取、account passphrase 获取等方法。
type KeyManager struct {
	keysDir    string    // ~/.assh/v2/data/keys 的展开绝对路径
	passphrase []byte    // account password（用于私钥加密），nil 表示未设置
	stdin      io.Reader // 用于 ReadKeyFromStdin，默认 os.Stdin
}

// New 创建并初始化 KeyManager。
// keysDir 为密钥存储目录路径（支持 ~/ 前缀），passphrase 为 account password。
// 调用者应确保在合适的时机将 passphrase 填充为零值。
func New(keysDir string, passphrase []byte) (*KeyManager, error) {
	expanded, err := config.ExpandPath(keysDir)
	if err != nil {
		return nil, fmt.Errorf("expand keys dir %q: %w", keysDir, err)
	}

	// 复制 passphrase，避免外部修改影响内部状态
	var pp []byte
	if len(passphrase) > 0 {
		pp = make([]byte, len(passphrase))
		copy(pp, passphrase)
	}

	return &KeyManager{
		keysDir:    expanded,
		passphrase: pp,
		stdin:      os.Stdin,
	}, nil
}

// GetAccountPassphrase 返回 account passphrase 的副本。
// 未设置 passphrase 时返回 nil（兼容模式）。
// 调用者在使用后应将返回的 []byte 填充为零值以防内存残留。
func (m *KeyManager) GetAccountPassphrase() []byte {
	if len(m.passphrase) == 0 {
		return nil
	}

	pp := make([]byte, len(m.passphrase))
	copy(pp, m.passphrase)
	return pp
}

// BackupPath 计算指定服务器名的备份快照路径并检查文件是否存在。
// 使用三层嵌套目录结构：ab/cd/{sha256hash}.pem（前4个字符拆分为两级目录）
// name 为服务器标识，用于生成 SHA256 哈希作为文件名。
// 返回完整的备份路径和是否存在状态。
func (m *KeyManager) BackupPath(name string) (path string, exists bool) {
	h := sha256.Sum256([]byte(name))
	fp := fmt.Sprintf("%x", h)
	// 构建三层嵌套目录结构：ab/cd/efghijklmnop.pem
	fpDir1 := fp[:2]
	fpDir2 := fp[2:4]
	fpFile := fp[4:]
	path = filepath.Join(m.keysDir, fpDir1, fpDir2, fpFile+".pem")
	if _, err := os.Stat(path); err == nil {
		return path, true
	}
	return path, false
}

// DeployPublicKey 暂未实现（P6.2-b 暂缓），提供桩实现以确保 port.KeyManager 兼容性。
func (m *KeyManager) DeployPublicKey(server *domain.Server, pubKey []byte) error {
	return errors.New("deploy public key is not yet implemented (P6.2-b deferred)")
}

// ReadKeyFromStdin 从标准输入读取密钥内容并保存到 data/keys/ 目录。
// 以 SHA256 内容指纹作为文件名（{fingerprint}.pem），权限 0600。
// 返回保存后的路径和指纹。
func (m *KeyManager) ReadKeyFromStdin() (savedPath string, fingerprint string, err error) {
	content, err := io.ReadAll(m.stdin)
	if err != nil {
		return "", "", fmt.Errorf("read stdin: %w", err)
	}
	if len(content) == 0 {
		return "", "", errors.New("empty input from stdin")
	}

	fp := fmt.Sprintf("%x", sha256.Sum256(content))
	destPath := filepath.Join(m.keysDir, fp+".pem")

	if err := config.EnsureDir(m.keysDir); err != nil {
		return "", "", fmt.Errorf("ensure keys dir: %w", err)
	}

	if err := os.WriteFile(destPath, content, 0600); err != nil {
		return "", "", fmt.Errorf("write key file: %w", err)
	}

	return destPath, fp, nil
}


