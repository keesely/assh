package assh

import (
	"assh/src/keygen"
	"assh/src/log"
	"fmt"
	"github.com/keesely/kiris"
)

func check(err error, msg string) {
	if err != nil {
		log.Fatalf("%s fail: %v", msg, err)
	}
}

func RsaEncrypt(content []byte, pubFile string) (output []byte, err error) {
	if !kiris.FileExists(pubFile) {
		err = fmt.Errorf("the public key file no exists.\n")
		return
	}
	pub, err := kiris.FileGetContents(pubFile)
	if err != nil {
		return
	}
	output, err = keygen.RsaEncrypt(content, pub)
	return
}

func RsaDecrypt(ciphertext []byte, privateFile string) (output []byte, err error) {
	if !kiris.FileExists(privateFile) {
		err = fmt.Errorf("the private key file no exists.\n")
		return
	}
	private, err := kiris.FileGetContents(privateFile)
	if err != nil {
		return
	}
	output, err = keygen.RsaDecrypt(ciphertext, private)
	return
}
