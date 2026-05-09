package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
)

const keyConfigKey = "aes_crypto_key"

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

func generateRandomKey(size int) ([]byte, error) {
	key := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

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

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (s *Store) decryptPassword(ciphertext []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", nil
	}

	key, err := s.getCryptoKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, data := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}