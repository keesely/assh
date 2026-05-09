package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

type RSAKey struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

func GenerateRSA(bits int) (*RSAKey, error) {
	if bits < 1024 || bits > 4096 {
		return nil, fmt.Errorf("invalid key size: %d (must be 1024-4096)", bits)
	}
	if bits%256 != 0 {
		return nil, fmt.Errorf("key size must be multiple of 256")
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	if err := privateKey.Validate(); err != nil {
		return nil, fmt.Errorf("key validation failed: %w", err)
	}

	return &RSAKey{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}, nil
}

func (k *RSAKey) ToPEMPrivateKey() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(k.PrivateKey),
	})
}

func (k *RSAKey) ToPEMPublicKey() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(k.PublicKey),
	})
}

func (k *RSAKey) ToOpenSSHPrivateKey() ([]byte, error) {
	block, err := ssh.MarshalPrivateKey(k.PrivateKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenSSH private key: %w", err)
	}
	if block == nil {
		return nil, errors.New("nil block returned")
	}
	return pem.EncodeToMemory(block), nil
}

func (k *RSAKey) ToOpenSSHPublicKey() ([]byte, error) {
	pubKey, err := ssh.NewPublicKey(k.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenSSH public key: %w", err)
	}
	if pubKey == nil {
		return nil, errors.New("nil public key returned")
	}
	return ssh.Marshal(pubKey), nil
}

func (k *RSAKey) WriteToFile(privatePath, publicPath string) error {
	privatePEM := k.ToPEMPrivateKey()
	err := os.WriteFile(privatePath, privatePEM, 0600)
	if err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	publicPEM := k.ToPEMPublicKey()
	err = os.WriteFile(publicPath, publicPEM, 0644)
	if err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	return nil
}

func ReadRSAKeyFromFile(privatePath, publicPath string) (*RSAKey, error) {
	privatePEM, err := os.ReadFile(privatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	publicPEM, err := os.ReadFile(publicPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	return RSAKeyFromPEM(privatePEM, publicPEM)
}

func RSAKeyFromPEM(privatePEM, publicPEM []byte) (*RSAKey, error) {
	privateKey, err := ParsePEMPrivateKey(privatePEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	publicKey, err := ParsePEMPublicKey(publicPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return &RSAKey{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

func ParsePEMPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM format")
	}

	if block.Type == "PRIVATE KEY" {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("not an RSA private key")
		}
		return rsaKey, nil
	}

	if block.Type == "RSA PRIVATE KEY" {
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}

	return nil, fmt.Errorf("unsupported PEM type: %s", block.Type)
}

func ParsePEMPublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM format")
	}

	if block.Type == "PUBLIC KEY" {
		pkixKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
		rsaKey, ok := pkixKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("not an RSA public key")
		}
		return rsaKey, nil
	}

	if block.Type == "RSA PUBLIC KEY" {
		return x509.ParsePKCS1PublicKey(block.Bytes)
	}

	return nil, fmt.Errorf("unsupported PEM type: %s", block.Type)
}

func RSAEncrypt(plain []byte, pubKey *rsa.PublicKey) ([]byte, error) {
	return rsa.EncryptPKCS1v15(rand.Reader, pubKey, plain)
}

func RSADecrypt(cipher []byte, privKey *rsa.PrivateKey) ([]byte, error) {
	return rsa.DecryptPKCS1v15(rand.Reader, privKey, cipher)
}

func RSAEncryptOAEP(plain []byte, pubKey *rsa.PublicKey) ([]byte, error) {
	return rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		pubKey,
		plain,
		nil,
	)
}

func RSADecryptOAEP(cipher []byte, privKey *rsa.PrivateKey) ([]byte, error) {
	return rsa.DecryptOAEP(
		sha256.New(),
		rand.Reader,
		privKey,
		cipher,
		nil,
	)
}
