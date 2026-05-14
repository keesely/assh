package keymgr

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"assh/config"
)

// BackupKey 将指定路径的密钥文件备份到 destPath（快照模式）。
//
// 约束：
//   - 源文件权限必须为 0600 或更严格（无 group/other 访问权限）。
//   - 权限过宽（group/other 可读）时返回错误。
//   - destPath 已存在时直接覆盖（始终反映最新快照）。
//
// 返回源文件内容的 SHA256 指纹。
func (m *KeyManager) BackupKey(srcPath string, destPath string) (fingerprint string, err error) {
	// 展开源路径
	absSrc, err := config.ExpandPath(srcPath)
	if err != nil {
		return "", fmt.Errorf("expand source path %q: %w", srcPath, err)
	}

	// 展开目标路径
	absDest, err := config.ExpandPath(destPath)
	if err != nil {
		return "", fmt.Errorf("expand dest path %q: %w", destPath, err)
	}

	// 检查源文件是否存在
	info, err := os.Stat(absSrc)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("source key file not found: %s", absSrc)
		}
		return "", fmt.Errorf("stat source file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("source path is a directory, not a key file: %s", absSrc)
	}

	// 检查权限：必须无 group/other 访问权限 (0077)
	if info.Mode().Perm()&0077 != 0 {
		return "", fmt.Errorf(
			"key file permissions %o are too permissive, must be 0600 or stricter: %s",
			info.Mode().Perm(), absSrc,
		)
	}

	// 读取文件内容
	content, err := os.ReadFile(absSrc)
	if err != nil {
		return "", fmt.Errorf("read source key file: %w", err)
	}
	if len(content) == 0 {
		return "", fmt.Errorf("source key file is empty: %s", absSrc)
	}

	// 计算 SHA256 指纹
	fp := fmt.Sprintf("%x", sha256.Sum256(content))

	// 确保目标目录存在
	if err := config.EnsureDir(filepath.Dir(absDest)); err != nil {
		return "", fmt.Errorf("ensure target directory: %w", err)
	}

	// 写入目标文件（0600 权限，覆盖已存在）
	if err := os.WriteFile(absDest, content, 0600); err != nil {
		return "", fmt.Errorf("write backup key file: %w", err)
	}

	return fp, nil
}
