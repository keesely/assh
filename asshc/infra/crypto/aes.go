package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

func GenerateAESKey(bits int) ([]byte, error) {
	switch bits {
	case 128:
		return generateRandomBytes(16)
	case 192:
		return generateRandomBytes(24)
	case 256:
		return generateRandomBytes(32)
	default:
		return nil, fmt.Errorf("invalid key size: %d (must be 128, 192, or 256)", bits)
	}
}

func generateRandomBytes(size int) ([]byte, error) {
	key := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return key, nil
}

func AESEncrypt(plaintext []byte, key []byte, mode string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	switch mode {
	case "CBC":
		return aesEncryptCBC(plaintext, block)
	case "CTR":
		return aesEncryptCTR(plaintext, block)
	case "GCM":
		return aesEncryptGCM(plaintext, key)
	case "ECB":
		return aesEncryptECB(plaintext, block)
	default:
		return nil, fmt.Errorf("unsupported encryption mode: %s (supported: CBC, CTR, GCM, ECB)", mode)
	}
}

func AESDecrypt(ciphertext []byte, key []byte, mode string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	switch mode {
	case "CBC":
		return aesDecryptCBC(ciphertext, block)
	case "CTR":
		return aesDecryptCTR(ciphertext, block)
	case "GCM":
		return aesDecryptGCM(ciphertext, key)
	case "ECB":
		return aesDecryptECB(ciphertext, block)
	default:
		return nil, fmt.Errorf("unsupported decryption mode: %s (supported: CBC, CTR, GCM, ECB)", mode)
	}
}

func aesEncryptCBC(plain []byte, block cipher.Block) ([]byte, error) {
	iv := make([]byte, block.BlockSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	plaintext := pkcs7Pad(plain, block.BlockSize())
	ciphertext := make([]byte, len(plaintext))

	encrypter := cipher.NewCBCEncrypter(block, iv)
	encrypter.CryptBlocks(ciphertext, plaintext)

	return append(iv, ciphertext...), nil
}

func aesDecryptCBC(ciphertext []byte, block cipher.Block) ([]byte, error) {
	blockSize := block.BlockSize()
	if len(ciphertext) < blockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := ciphertext[:blockSize]
	data := ciphertext[blockSize:]

	if len(data)%blockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of block size")
	}

	plaintext := make([]byte, len(data))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, data)

	return pkcs7Unpad(plaintext)
}

func aesEncryptCTR(plain []byte, block cipher.Block) ([]byte, error) {
	iv := make([]byte, block.BlockSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	ciphertext := make([]byte, len(plain))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext, plain)

	return append(iv, ciphertext...), nil
}

func aesDecryptCTR(ciphertext []byte, block cipher.Block) ([]byte, error) {
	blockSize := block.BlockSize()
	if len(ciphertext) < blockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := ciphertext[:blockSize]
	data := ciphertext[blockSize:]

	plaintext := make([]byte, len(data))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(plaintext, data)

	return plaintext, nil
}

func aesEncryptGCM(plain []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plain, nil)
	return ciphertext, nil
}

func aesDecryptGCM(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, data := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

func aesEncryptECB(plain []byte, block cipher.Block) ([]byte, error) {
	plaintext := pkcs7Pad(plain, block.BlockSize())
	ciphertext := make([]byte, len(plaintext))

	for i := 0; i < len(plaintext); i += block.BlockSize() {
		block.Encrypt(ciphertext[i:i+block.BlockSize()], plaintext[i:i+block.BlockSize()])
	}

	return ciphertext, nil
}

func aesDecryptECB(cipher []byte, block cipher.Block) ([]byte, error) {
	if len(cipher)%block.BlockSize() != 0 {
		return nil, errors.New("ciphertext is not a multiple of block size")
	}

	plaintext := make([]byte, len(cipher))
	for i := 0; i < len(cipher); i += block.BlockSize() {
		block.Decrypt(plaintext[i:i+block.BlockSize()], cipher[i:i+block.BlockSize()])
	}

	return pkcs7Unpad(plaintext)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	return append(data, padtext...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	padding := int(data[len(data)-1])
	if padding > len(data) || padding == 0 {
		return nil, errors.New("invalid padding")
	}

	for i := len(data) - padding; i < len(data); i++ {
		if int(data[i]) != padding {
			return nil, errors.New("invalid padding")
		}
	}

	return data[:len(data)-padding], nil
}
