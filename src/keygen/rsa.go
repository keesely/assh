// rsa.go kee > 2019/11/12

package keygen

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

// 生成RSA私钥
func NewRsa(bits int) (*Keygen, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}

	block := PemBlock{}

	block.Private = &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	var private bytes.Buffer
	if err := pem.Encode(&private, block.Private); err != nil {
		return nil, err
	}

	public, err := RsaPublicKey(&privateKey.PublicKey, &block)
	if err != nil {
		return nil, err
	}

	return &Keygen{
		Type:       "rsa",
		PrivatePem: &private,
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
		PublicPem:  public,
		Block:      &block,
	}, nil
}

// 生成RSA公钥
func RsaPublicKey(publicKey *rsa.PublicKey, block *PemBlock) (*bytes.Buffer, error) {
	derPkix, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	block.Public = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: derPkix,
	}

	var public bytes.Buffer
	if err = pem.Encode(&public, block.Public); err != nil {
		return nil, err
	}
	return &public, err
}

func RsaEncrypt(origin []byte, publicKey []byte) ([]byte, error) {
	block, _ := pem.Decode(publicKey)
	if block == nil {
		return nil, errors.New("public key error")
	}

	// 解析公钥
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	// 类型断言
	pub := pubInterface.(*rsa.PublicKey)
	// 加密
	return rsa.EncryptPKCS1v15(rand.Reader, pub, origin)
}

func RsaDecrypt(ciphertext []byte, privateKey []byte) ([]byte, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, errors.New("private key error")
	}
	private, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return rsa.DecryptPKCS1v15(rand.Reader, private, ciphertext)
}
