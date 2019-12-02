package asshc

import (
	"assh/asshc/keygen"
	"assh/log"
	"fmt"
	"github.com/keesely/kiris"
	"os"
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

// copy ...
func CopyFile(srcFile, dstFile string) error {
	if !kiris.FileExists(srcFile) {
		return fmt.Errorf("file not found: %s\n", srcFile)
	}
	src, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer dst.Close()

	var buf = make([]byte, 2048)
	for {
		n, err := src.Read(buf)
		if err != nil {
			break
		}
		dst.Write(buf[:n])
	}
	return nil
}
