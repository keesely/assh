// keygen.go kee > 2019/11/13

package keygen

import (
	"bytes"
	"encoding/pem"
	"fmt"
	"golang.org/x/crypto/ssh"
	"os"
	"os/user"
	"strings"
)

type PemBlock struct {
	Private *pem.Block
	Public  *pem.Block
}

type Keygen struct {
	Type       string
	PrivateKey interface{}
	PrivatePem *bytes.Buffer
	PublicKey  interface{}
	PublicPem  *bytes.Buffer
	Block      *PemBlock
}

// 生成`ssh`公/私钥
// return public Key, private Key, error
func (k *Keygen) GenSSHKey(comment string) (string, string, error) {
	pub, err := ssh.NewPublicKey(k.PublicKey)
	if err != nil {
		return "", "", err
	}
	public := ssh.MarshalAuthorizedKey(pub)

	pubStr := string(public)
	if comment == "" {
		if u, err := user.Current(); err == nil {
			hostname, _ := os.Hostname()
			uName := u.Username
			comment = uName + "@" + hostname
		}
	}
	pubStr = strings.Replace(pubStr, "\n", "", -1)
	pubStr = fmt.Sprintf("%s %s", pubStr, comment)
	return pubStr, k.PrivatePem.String(), nil
}

// 生成密钥对
// return private pem and public pem
func (k *Keygen) GenPem() (string, string) {
	return k.PublicPem.String(), k.PrivatePem.String()
}
