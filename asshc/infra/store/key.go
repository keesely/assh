package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"

	"assh/asshc/infra/crypto"
)

// keyConfigKey 是 config 表中用于存储 AES 加密密钥的键名。
const keyConfigKey = "aes_crypto_key"

// generateAndStoreKey 生成 256 位（32 字节）随机 AES 密钥，
// 经过 Base64 编码后存入 config 表，同时缓存在内存中。
func (s *Store) generateAndStoreKey() error {
	key, err := generateRandomKey(32)
	if err != nil {
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(key)
	_, err = s.db.Exec("INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)", keyConfigKey, encoded)
	if err == nil {
		s.cryptoKey = key
	}
	return err
}

// getCryptoKey 获取 AES 加密密钥，优先从内存缓存中读取。
// 如果缓存为空，从数据库 config 表中查询并缓存。
// 如果表中没有密钥记录，返回 nil, nil。
func (s *Store) getCryptoKey() ([]byte, error) {
	if s.cryptoKey != nil {
		return s.cryptoKey, nil
	}

	var encoded string
	err := s.db.QueryRow("SELECT value FROM config WHERE key = ?", keyConfigKey).Scan(&encoded)
	if err == nil {
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err == nil {
			s.cryptoKey = key
		}
		return key, err
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return nil, err
}

// generateRandomKey 生成指定长度的密码学安全随机字节序列。
// size 为所需字节数（如 32 对应 AES-256）。
func generateRandomKey(size int) ([]byte, error) {
	key := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// encryptPassword 使用 AES-GCM 对密码明文进行加密。
// 如果尚未生成加密密钥，会自动生成并持久化。
// 返回的密文格式为：nonce（12 字节）+ ciphertext。
func (s *Store) encryptPassword(plaintext string) ([]byte, error) {
	if plaintext == "" {
		return []byte{}, nil
	}

	key, err := s.getCryptoKey()
	if err != nil {
		return nil, err
	}
	if key == nil {
		if err := s.generateAndStoreKey(); err != nil {
			return nil, err
		}
		key, err = s.getCryptoKey()
		if err != nil {
			return nil, err
		}
	}

	return crypto.EncryptGCM(key, []byte(plaintext))
}

// decryptPassword 使用 AES-GCM 解密密文，恢复密码明文。
// 密文格式必须为：nonce（12 字节）+ ciphertext + tag。
func (s *Store) decryptPassword(ciphertext []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", nil
	}

	key, err := s.getCryptoKey()
	if err != nil {
		return "", err
	}

	plaintext, err := crypto.DecryptGCM(key, ciphertext)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}