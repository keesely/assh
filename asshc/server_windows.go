package asshc

import (
	"assh/log"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	//"os/signal"
	//"runtime"
	"golang.org/x/sys/windows"
	//"syscall"
)

//监听终端变化
func listenWindowChange(session *ssh.Session, fd int) {
	go func() {
		sigWinCh := make(chan os.Signal, 1)
		defer close(sigWinCh)
		//signal.Notify(sigWinCh, syscall.SIGWINCH)

		//fd := int(os.Stdin.Fd())
		//fd := getStdoutFd()
		fd := int(os.Stdout.Fd())
		termW, termH, err := terminal.GetSize(fd)
		if err != nil {
			log.Fatal(err)
		}

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

func getStdinFd() int {
	return int(windows.Stdin)
}

func getStdoutFd() int {
	return int(windows.Stdout)
}
