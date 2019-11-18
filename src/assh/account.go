// account.go kee > 2019/11/14

package assh

import (
	key "assh/src/keygen"
	//"fmt"
	"github.com/keesely/kiris"
	"github.com/keesely/kiris/hash"
	"log"
	"os"
)

var (
	cPath    = kiris.RealPath("~/.assh")
	pemFile  = cPath + "/key_rsa"
	pubFile  = cPath + "/key_rsa.pub"
	passFile = cPath + "/account.key"
)

func init() {
	if !kiris.IsDir(cPath) {
		// 创建配置目录
		if err := os.MkdirAll(cPath, os.ModePerm); err != nil {
			log.Fatalf("mkdir %s fail", cPath, err.Error())
		}
	}

	// 密钥文件生成
	if !kiris.FileExists(pemFile) && !kiris.FileExists(pubFile) {
		rsa, err := key.NewRsa(2048)
		if err != nil {
			log.Fatal(err)
		}
		public, private := rsa.GenPem()
		if err = kiris.FilePutContents(pubFile, public, 0); err != nil {
			log.Fatal(err)
		}
		if err = kiris.FilePutContents(pemFile, private, 0); err != nil {
			log.Fatal(err)
		}
	}

}

func GetPasswd() string {
	// 判断是否存在密码文件
	if !kiris.FileExists(passFile) {
		log.Fatal("You have not set the password.")
	}

	ciphertext, err := kiris.FileGetContents(passFile)
	if err != nil {
		log.Fatal(err)
	}

	privateKey, err := kiris.FileGetContents(pemFile)
	if err != nil {
		log.Fatal(err)
	}
	decrypt, err := key.RsaDecrypt(ciphertext, privateKey)
	if err != nil {
		log.Fatal(err)
	}
	return string(decrypt)
}

func SetPasswd(passwd string) {
	if passwd == "" {
		log.Fatal("The password is empty")
	}
	publicKey, err := kiris.FileGetContents(pubFile)
	if err != nil {
		log.Fatal(err)
	}
	passwd = hash.Md5(passwd)
	encrypt, err := key.RsaEncrypt([]byte(passwd), publicKey)
	if err != nil {
		log.Fatal(err)
	}
	kiris.FilePutContents(passFile, string(encrypt), 0)
}
