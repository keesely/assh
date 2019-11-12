// keygen.go kee > 2019/11/12

package keygen

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"golang.org/x/crypto/ssh"
	"os"
	"os/user"
	"strings"
)

func GenRsaKey(bits int) error {
	priKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return err
	}
	derStream := x509.MarshalPKCS1PrivateKey(priKey)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: derStream,
	}

	file, err := os.Create("private.pem")
	if err != nil {
		return err
	}

	err = pem.Encode(file, block)
	if err != nil {
		return err
	}

	// 生成公钥
	pubKey := &priKey.PublicKey
	derPkix, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return err
	}
	block = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: derPkix,
	}
	file, err = os.Create("public.pem")
	if err != nil {
		return err
	}
	err = pem.Encode(file, block)
	return err
}

// 生成RSA私钥
func RsaPrivateKey(bits int) (*bytes.Buffer, *rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	var private bytes.Buffer
	if err := pem.Encode(&private, block); err != nil {
		return nil, nil, err
	}
	return &private, privateKey, nil
}

// 生成RSA公钥
func RsaPublicKey(privateKey *rsa.PrivateKey) (*bytes.Buffer, *rsa.PublicKey, error) {
	pubKey := &privateKey.PublicKey
	derPkix, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, nil, err
	}

	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: derPkix,
	}

	var public bytes.Buffer
	if err = pem.Encode(&public, block); err != nil {
		return nil, nil, err
	}
	return &public, pubKey, err
}

// 生成`ssh`公/私钥
// return public Key, private Key, error
func SshKeygen(private *bytes.Buffer, privateKey *rsa.PrivateKey) (string, string, error) {
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	public := ssh.MarshalAuthorizedKey(pub)

	pubStr := string(public)
	if u, err := user.Current(); err == nil {
		hostname, _ := os.Hostname()
		uName := u.Username
		pubStr = strings.Replace(pubStr, "\n", "", -1)
		pubStr = fmt.Sprintf("%s %s@%s \n", pubStr, uName, hostname)
	}
	return pubStr, private.String(), nil
}
