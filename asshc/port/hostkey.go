package port

import (
	"net"

	"golang.org/x/crypto/ssh"
)

// HostKeyCallback 是 SSH HostKey 校验回调函数的类型定义。
// hostname 为连接的服务器地址，remote 为远程网络地址，key 为服务器返回的公钥。
// 返回 nil 表示校验通过，返回 error 表示拒绝连接。
type HostKeyCallback func(hostname string, remote net.Addr, key ssh.PublicKey) error

// HostKeyChecker 定义 HostKey 校验器接口。
// 实现该接口的类型可以集成 known_hosts 文件校验或其他自定义校验逻辑。
type HostKeyChecker interface {
	// Check 检查远程服务器的 HostKey 是否可信。
	Check(hostname string, remote net.Addr, key ssh.PublicKey) error
}
