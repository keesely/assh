package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

// Ed25519Key 封装 Ed25519 密钥对。
// Ed25519 是 Edwards 曲线的数字签名算法，密钥短且性能高。
type Ed25519Key struct {
	PrivateKey ed25519.PrivateKey // Ed25519 私钥（64 字节）
	PublicKey  ed25519.PublicKey  // Ed25519 公钥（32 字节）
}

// GenerateEd25519 生成 Ed25519 密钥对。
func GenerateEd25519() (*Ed25519Key, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key: %w", err)
	}

	return &Ed25519Key{
		PrivateKey: privKey,
		PublicKey:  pubKey,
	}, nil
}

// ToPEMPrivateKey 将 Ed25519 私钥编码为 PKCS#8 PEM 格式。
func (k *Ed25519Key) ToPEMPrivateKey() []byte {
	privBytes, _ := x509.MarshalPKCS8PrivateKey(k.PrivateKey)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	})
}

// ToPEMPublicKey 将 Ed25519 公钥编码为 PKIX PEM 格式。
func (k *Ed25519Key) ToPEMPublicKey() []byte {
	pubBytes, _ := x509.MarshalPKIXPublicKey(k.PublicKey)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})
}

// ToOpenSSHPrivateKey 将 Ed25519 私钥编码为 OpenSSH 格式 PEM。
func (k *Ed25519Key) ToOpenSSHPrivateKey() ([]byte, error) {
	block, err := ssh.MarshalPrivateKey(k.PrivateKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenSSH private key: %w", err)
	}
	if block == nil {
		return nil, errors.New("nil block returned")
	}
	return pem.EncodeToMemory(block), nil
}

// ToOpenSSHPublicKey 将 Ed25519 公钥编码为 OpenSSH authorized_keys 格式。
func (k *Ed25519Key) ToOpenSSHPublicKey() ([]byte, error) {
	pubKey, err := ssh.NewPublicKey(k.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenSSH public key: %w", err)
	}
	if pubKey == nil {
		return nil, errors.New("nil public key returned")
	}
	return ssh.Marshal(pubKey), nil
}

// WriteToFile 将 Ed25519 密钥对写入文件（私钥 0600，公钥 0644）。
func (k *Ed25519Key) WriteToFile(privatePath, publicPath string) error {
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

// ReadEd25519KeyFromFile 从文件读取 Ed25519 密钥对。
func ReadEd25519KeyFromFile(privatePath, publicPath string) (*Ed25519Key, error) {
	privatePEM, err := os.ReadFile(privatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	publicPEM, err := os.ReadFile(publicPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	return Ed25519KeyFromPEM(privatePEM, publicPEM)
}

// Ed25519KeyFromPEM 从 PEM 数据解析 Ed25519 密钥对。
func Ed25519KeyFromPEM(privatePEM, publicPEM []byte) (*Ed25519Key, error) {
	privateKey, err := ParseEd25519PrivateKey(privatePEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	publicKey, err := ParseEd25519PublicKey(publicPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return &Ed25519Key{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// ParseEd25519PrivateKey 从 PEM 数据解析 Ed25519 私钥（PKCS#8 格式）。
func ParseEd25519PrivateKey(data []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM format")
	}

	if block.Type == "PRIVATE KEY" {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
		}
		edKey, ok := key.(ed25519.PrivateKey)
		if !ok {
			return nil, errors.New("not an Ed25519 private key")
		}
		return edKey, nil
	}

	return nil, fmt.Errorf("unsupported PEM type: %s", block.Type)
}

// ParseEd25519PublicKey 从 PEM 数据解析 Ed25519 公钥（PKIX 格式）。
func ParseEd25519PublicKey(data []byte) (ed25519.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM format")
	}

	pkixKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	edKey, ok := pkixKey.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("not an Ed25519 public key")
	}
	return edKey, nil
}
