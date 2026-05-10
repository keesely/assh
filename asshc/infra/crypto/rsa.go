// Package crypto 提供非对称加密（RSA/ECDSA/Ed25519）和对称加密（AES）的实现。
//
// 支持 RSA 密钥对生成、PEM/OpenSSH 格式序列化、PKCS1v15/OAEP 加解密；
// 支持 Ed25519 和 ECDSA（P256/P384/P521）密钥对生成与序列化；
// 支持 AES 多种模式（CBC/CTR/GCM/ECB）的加解密。
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

// RSAKey 封装 RSA 密钥对，包含私钥和公钥。
type RSAKey struct {
	PrivateKey *rsa.PrivateKey // RSA 私钥
	PublicKey  *rsa.PublicKey  // RSA 公钥
}

// GenerateRSA 生成指定位数的 RSA 密钥对。
// bits 必须是 1024-4096 之间的 256 的倍数。
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

// ToPEMPrivateKey 将 RSA 私钥编码为 PKCS#1 PEM 格式。
func (k *RSAKey) ToPEMPrivateKey() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(k.PrivateKey),
	})
}

// ToPEMPublicKey 将 RSA 公钥编码为 PKCS#1 PEM 格式。
func (k *RSAKey) ToPEMPublicKey() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(k.PublicKey),
	})
}

// ToOpenSSHPrivateKey 将 RSA 私钥编码为 OpenSSH 格式 PEM。
// 返回的密钥未加密（空密码）。
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

// ToOpenSSHPublicKey 将 RSA 公钥编码为 OpenSSH authorized_keys 格式。
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

// WriteToFile 将 RSA 密钥对写入指定文件路径。
// 私钥文件权限设为 0600，公钥文件权限设为 0644。
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

// ReadRSAKeyFromFile 从文件读取 RSA 密钥对。
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

// RSAKeyFromPEM 从 PEM 数据解析 RSA 密钥对。
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

// ParsePEMPrivateKey 从 PEM 数据解析 RSA 私钥。
// 支持 PKCS#1（"RSA PRIVATE KEY"）和 PKCS#8（"PRIVATE KEY"）两种格式。
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

// ParsePEMPublicKey 从 PEM 数据解析 RSA 公钥。
// 支持 PKIX（"PUBLIC KEY"）和 PKCS#1（"RSA PUBLIC KEY"）两种格式。
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

// RSAEncrypt 使用 PKCS#1 v1.5 填充方式加密数据。
func RSAEncrypt(plain []byte, pubKey *rsa.PublicKey) ([]byte, error) {
	return rsa.EncryptPKCS1v15(rand.Reader, pubKey, plain)
}

// RSADecrypt 使用 PKCS#1 v1.5 填充方式解密数据。
func RSADecrypt(cipher []byte, privKey *rsa.PrivateKey) ([]byte, error) {
	return rsa.DecryptPKCS1v15(rand.Reader, privKey, cipher)
}

// RSAEncryptOAEP 使用 OAEP 填充方式（SHA-256）加密数据，安全性更高。
func RSAEncryptOAEP(plain []byte, pubKey *rsa.PublicKey) ([]byte, error) {
	return rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		pubKey,
		plain,
		nil,
	)
}

// RSADecryptOAEP 使用 OAEP 填充方式（SHA-256）解密数据。
func RSADecryptOAEP(cipher []byte, privKey *rsa.PrivateKey) ([]byte, error) {
	return rsa.DecryptOAEP(
		sha256.New(),
		rand.Reader,
		privKey,
		cipher,
		nil,
	)
}
