package port

type Cryptor interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
	EncryptWithPassword(plaintext []byte, password string) ([]byte, error)
	DecryptWithPassword(ciphertext []byte, password string) ([]byte, error)
}
