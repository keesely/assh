package ssh

import (
	"net"

	"golang.org/x/crypto/ssh"
)

// KnownHostsChecker 基于 known_hosts 文件的 HostKey 校验器（预留实现）。
// 当前版本为占位实现，Check 方法始终返回 nil（不校验）。
type KnownHostsChecker struct {
	knownHostsPath string // known_hosts 文件路径（预留）
}

// NewKnownHostsChecker 创建 KnownHostsChecker 实例（预留）。
func NewKnownHostsChecker(path string) *KnownHostsChecker {
	return &KnownHostsChecker{knownHostsPath: path}
}

// Check 校验远程主机的 HostKey（当前为占位实现，始终通过）。
func (k *KnownHostsChecker) Check(hostname string, remote net.Addr, key ssh.PublicKey) error {
	return nil
}

// InsecureChecker 不安全的 HostKey 校验器，始终通过所有校验。
type InsecureChecker struct{}

// NewInsecureChecker 创建 InsecureChecker 实例。
func NewInsecureChecker() *InsecureChecker {
	return &InsecureChecker{}
}

// Check 不做任何校验，始终返回 nil。
func (i *InsecureChecker) Check(hostname string, remote net.Addr, key ssh.PublicKey) error {
	return nil
}
