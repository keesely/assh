package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

// ECDSAKey 封装 ECDSA 密钥对，支持 P-256、P-384、P-521 三种曲线。
type ECDSAKey struct {
	PrivateKey *ecdsa.PrivateKey // ECDSA 私钥
	PublicKey  *ecdsa.PublicKey  // ECDSA 公钥
}

// GenerateECDSA 在指定椭圆曲线上生成 ECDSA 密钥对。
func GenerateECDSA(curve elliptic.Curve) (*ECDSAKey, error) {
	privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ECDSA key: %w", err)
	}

	return &ECDSAKey{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}, nil
}

// GenerateP256ECDSA 在 P-256（secp256r1）曲线上生成 ECDSA 密钥对。
func GenerateP256ECDSA() (*ECDSAKey, error) {
	return GenerateECDSA(elliptic.P256())
}

// GenerateP384ECDSA 在 P-384（secp384r1）曲线上生成 ECDSA 密钥对。
func GenerateP384ECDSA() (*ECDSAKey, error) {
	return GenerateECDSA(elliptic.P384())
}

// GenerateP521ECDSA 在 P-521（secp521r1）曲线上生成 ECDSA 密钥对。
func GenerateP521ECDSA() (*ECDSAKey, error) {
	return GenerateECDSA(elliptic.P521())
}

// ToPEMPrivateKey 将 ECDSA 私钥编码为 SEC.1 / PKCS#8 PEM 格式。
func (k *ECDSAKey) ToPEMPrivateKey() []byte {
	privBytes, _ := x509.MarshalECPrivateKey(k.PrivateKey)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privBytes,
	})
}

// ToPEMPublicKey 将 ECDSA 公钥编码为 PKIX PEM 格式。
func (k *ECDSAKey) ToPEMPublicKey() []byte {
	pubBytes, _ := x509.MarshalPKIXPublicKey(k.PublicKey)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})
}

// ToOpenSSHPrivateKey 将 ECDSA 私钥编码为 OpenSSH 格式 PEM。
func (k *ECDSAKey) ToOpenSSHPrivateKey() ([]byte, error) {
	block, err := ssh.MarshalPrivateKey(k.PrivateKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenSSH private key: %w", err)
	}
	if block == nil {
		return nil, errors.New("nil block returned")
	}
	return pem.EncodeToMemory(block), nil
}

// ToOpenSSHPublicKey 将 ECDSA 公钥编码为 OpenSSH authorized_keys 格式。
func (k *ECDSAKey) ToOpenSSHPublicKey() ([]byte, error) {
	pubKey, err := ssh.NewPublicKey(k.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenSSH public key: %w", err)
	}
	if pubKey == nil {
		return nil, errors.New("nil public key returned")
	}
	return ssh.Marshal(pubKey), nil
}

// WriteToFile 将 ECDSA 密钥对写入文件（私钥 0600，公钥 0644）。
func (k *ECDSAKey) WriteToFile(privatePath, publicPath string) error {
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

// ReadECDSAKeyFromFile 从文件读取 ECDSA 密钥对。
func ReadECDSAKeyFromFile(privatePath, publicPath string) (*ECDSAKey, error) {
	privatePEM, err := os.ReadFile(privatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	publicPEM, err := os.ReadFile(publicPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	return ECDSAKeyFromPEM(privatePEM, publicPEM)
}

// ECDSAKeyFromPEM 从 PEM 数据解析 ECDSA 密钥对。
func ECDSAKeyFromPEM(privatePEM, publicPEM []byte) (*ECDSAKey, error) {
	privateKey, err := ParseECDSAPrivateKey(privatePEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	publicKey, err := ParseECDSAPublicKey(publicPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return &ECDSAKey{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// ParseECDSAPrivateKey 从 PEM 数据解析 ECDSA 私钥。
// 支持 "EC PRIVATE KEY"（SEC.1）、"PRIVATE KEY"（PKCS#8）和 "ECDSA PRIVATE KEY" 格式。
func ParseECDSAPrivateKey(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM format")
	}

	if block.Type == "EC PRIVATE KEY" || block.Type == "PRIVATE KEY" {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
		}
		ecdsaKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("not an ECDSA private key")
		}
		return ecdsaKey, nil
	}

	if block.Type == "ECDSA PRIVATE KEY" {
		return x509.ParseECPrivateKey(block.Bytes)
	}

	return nil, fmt.Errorf("unsupported PEM type: %s", block.Type)
}

// ParseECDSAPublicKey 从 PEM 数据解析 ECDSA 公钥（PKIX 格式）。
func ParseECDSAPublicKey(data []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM format")
	}

	pkixKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	ecdsaKey, ok := pkixKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("not an ECDSA public key")
	}
	return ecdsaKey, nil
}
