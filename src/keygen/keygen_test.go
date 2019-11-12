// keygen_test.go kee > 2019/11/12

package keygen

import (
	"fmt"
	"testing"
)

func TestKeygen(t *testing.T) {
	//RsaKeyGen(1024)
	//RsaKeyGen(2048)
	private, priKey, _ := RsaPrivateKey(2048)
	pub, pri, err := SshKeygen(private, priKey)
	fmt.Println(pub, "\n", pri, err)
}
