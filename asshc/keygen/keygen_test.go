// keygen_test.go kee > 2019/11/12

package keygen

import (
	"fmt"
	"testing"
)

func TestKeygen(t *testing.T) {
	keygen, _ := NewRsa(2048)
	pub, pri, _ := keygen.GenSshKey()
	fmt.Println(pub, "\n", pri)

	public, private := keygen.GenPem()
	fmt.Println(public, "\n", private)
}
