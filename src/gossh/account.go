// account.go kee > 2019/11/14

package gossh

import (
	"github.com/keesely/kiris"
)

var (
	cPath = kiris.RealPath("~/.gossh")
	pem   = "key_rsa"
	pub   = "key_rsa.pub"
)

func init() {

}
