package port

// Cryptor 定义加解密接口，支持无密码加密和基于密码的加密两种模式。
// 无密码模式使用随机生成的内部密钥，基于密码的加密使用用户提供的密码派生密钥。
type Cryptor interface {
	// Encrypt 使用内部密钥加密明文数据。
	Encrypt(plaintext []byte) ([]byte, error)
	// Decrypt 使用内部密钥解密密文数据。
	Decrypt(ciphertext []byte) ([]byte, error)
	// EncryptWithPassword 使用密码派生的密钥加密明文数据。
	EncryptWithPassword(plaintext []byte, password string) ([]byte, error)
	// DecryptWithPassword 使用密码派生的密钥解密密文数据。
	DecryptWithPassword(ciphertext []byte, password string) ([]byte, error)
}
