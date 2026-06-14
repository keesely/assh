//go:build windows

package ssh

// watchResize Windows 系统不支持 SIGWINCH 信号，终端大小变化需要手动触发。
// 此函数为占位实现，Windows 下终端大小调整功能不可用。
func watchResize(fd int, resizeFn func(w, h int)) {
	// Windows 不支持 SIGWINCH，终端大小变化不会自动通知
	// 用户可以手动使用 resize 命令调整
}
