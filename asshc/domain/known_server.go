package domain

import (
	"crypto/sha256"
	"fmt"
)

// KnownServer 表示一个未具名服务器的直连记录（隐性表）。
// 用于记录直连（user@host / -H host）的连接历史和密钥关联。
type KnownServer struct {
	ID              string // SHA256(user@host:port#authFingerprint)
	Host            string
	Port            int
	User            string
	AuthFingerprint string // SHA256(password) 或 SHA256(keypath) 或组合
	KeyBackupPath   string // data/keys/ 中关联的密钥备份路径
	LastConnectedAt string
	ConnectCount    int
	CreatedAt       string
	UpdatedAt       string
}

// ComputeKnownServerID 计算 known_servers 表的 ID。
// 格式：SHA256(fmt.Sprintf("%s@%s:%d#%s", user, host, port, authFingerprint))
func ComputeKnownServerID(user, host string, port int, authFingerprint string) string {
	raw := fmt.Sprintf("%s@%s:%d#%s", user, host, port, authFingerprint)
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h)
}

// ComputeAuthFingerprint 根据认证方式计算指纹。
// 密码: SHA256(password)
// 密钥: SHA256(绝对路径)
// 两者: SHA256(password):SHA256(keypath)
// 仅 agent: 空字符串
func ComputeAuthFingerprint(password, keyFile string) string {
	if password != "" && keyFile != "" {
		pwHash := sha256.Sum256([]byte(password))
		keyHash := sha256.Sum256([]byte(keyFile))
		return fmt.Sprintf("%x:%x", pwHash, keyHash)
	}
	if password != "" {
		h := sha256.Sum256([]byte(password))
		return fmt.Sprintf("%x", h)
	}
	if keyFile != "" {
		h := sha256.Sum256([]byte(keyFile))
		return fmt.Sprintf("%x", h)
	}
	return ""
}