package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"sync"
)

// authHMACKey is the HMAC key used for auth fingerprint computation.
// Initialized with a default value; should be overridden by the application
// with a cryptographically random key stored in the config database.
var (
	authHMACKey     []byte
	authHMACKeyOnce sync.Once
)

// InitAuthHMACKey sets the HMAC key for auth fingerprint computation.
// Should be called once during application initialization with a key from the config DB.
// If not called, a default key is used (less secure but backward-compatible).
func InitAuthHMACKey(key []byte) {
	authHMACKeyOnce.Do(func() {
		authHMACKey = make([]byte, len(key))
		copy(authHMACKey, key)
	})
}

// getHMACKey returns the current HMAC key, initializing with a default if needed.
func getHMACKey() []byte {
	authHMACKeyOnce.Do(func() {
		// Default key — callers should override via InitAuthHMACKey()
		authHMACKey = []byte("assh-auth-fingerprint-hmac-key-v2")
	})
	return authHMACKey
}

// hmacSHA256 computes HMAC-SHA256 of data using the configured key.
func hmacSHA256(data []byte) [32]byte {
	mac := hmac.New(sha256.New, getHMACKey())
	mac.Write(data)
	var result [32]byte
	copy(result[:], mac.Sum(nil))
	return result
}

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
// 格式：HMAC-SHA256(fmt.Sprintf("%s@%s:%d#%s", user, host, port, authFingerprint))
func ComputeKnownServerID(user, host string, port int, authFingerprint string) string {
	raw := fmt.Sprintf("%s@%s:%d#%s", user, host, port, authFingerprint)
	h := hmacSHA256([]byte(raw))
	return fmt.Sprintf("%x", h)
}

// ComputeAuthFingerprint 根据认证方式计算指纹。
// 使用 HMAC-SHA256 代替普通 SHA256，防止彩虹表攻击。
// 密码: HMAC-SHA256(password)
// 密钥: HMAC-SHA256(绝对路径)
// 两者: HMAC-SHA256(password):HMAC-SHA256(keypath)
// 仅 agent: 空字符串
func ComputeAuthFingerprint(password, keyFile string) string {
	if password != "" && keyFile != "" {
		pwHash := hmacSHA256([]byte(password))
		keyHash := hmacSHA256([]byte(keyFile))
		return fmt.Sprintf("%x:%x", pwHash, keyHash)
	}
	if password != "" {
		h := hmacSHA256([]byte(password))
		return fmt.Sprintf("%x", h)
	}
	if keyFile != "" {
		h := hmacSHA256([]byte(keyFile))
		return fmt.Sprintf("%x", h)
	}
	return ""
}