// +build !windows

package assh

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"os/signal"
	//"runtime"
	"syscall"
)

//监听终端变化
func listenWindowChange(session *ssh.Session, fd int) {
	go func() {
		sigWinCh := make(chan os.Signal, 1)
		defer close(sigWinCh)

		//if runtime.GOOS != "windows" {
		signal.Notify(sigWinCh, syscall.SIGWINCH)
		//}
		termW, termH, _ := terminal.GetSize(fd)
		//if e != nil {
		//check(e, "assh > terminal window resize")
		//}

		for {
			select {
			case swc := <-sigWinCh:
				if swc == nil {
					return
				}
				curTermW, curTermH, e := terminal.GetSize(fd)

				//确认窗口改变
				if curTermH == termH && curTermW == termW {
					continue
				}
				// 更新端窗口
				session.WindowChange(curTermH, curTermW)

				if e != nil {
					continue
				}
				termW, termH = curTermW, curTermH
			}
		}
	}()
}
