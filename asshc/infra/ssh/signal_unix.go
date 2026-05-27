//go:build !windows

package ssh

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

// watchResize 监听终端窗口大小变化信号（SIGWINCH），实时调整远程 PTY 尺寸。
// 仅在 Unix 系统（darwin/linux）上可用。
func watchResize(fd int, resizeFn func(w, h int)) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)

	for range sigCh {
		w, h := getTerminalSize(fd)
		resizeFn(w, h)
	}
}

// getTerminalSize 获取终端窗口大小。
func getTerminalSize(fd int) (int, int) {
	w, h, err := term.GetSize(fd)
	if err != nil {
		return 80, 24
	}
	return w, h
}
