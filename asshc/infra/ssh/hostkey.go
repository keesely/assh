package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// KnownHostsChecker 基于 known_hosts 文件的 HostKey 校验器。
type KnownHostsChecker struct {
	knownHostsPath string
	callback       ssh.HostKeyCallback
}

// NewKnownHostsChecker 创建 KnownHostsChecker 实例。
// path 为 known_hosts 文件路径，支持 ~ 展开。
func NewKnownHostsChecker(path string) (*KnownHostsChecker, error) {
	// 展开 ~ 路径
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create known_hosts directory: %w", err)
	}

	// 如果文件不存在，创建空文件
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(""), 0600); err != nil {
			return nil, fmt.Errorf("failed to create known_hosts file: %w", err)
		}
	}

	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load known_hosts: %w", err)
	}

	return &KnownHostsChecker{
		knownHostsPath: path,
		callback:       callback,
	}, nil
}

// Check 校验远程主机的 HostKey。
func (k *KnownHostsChecker) Check(hostname string, remote net.Addr, key ssh.PublicKey) error {
	return k.callback(hostname, remote, key)
}

// GetPath 返回 known_hosts 文件路径。
func (k *KnownHostsChecker) GetPath() string {
	return k.knownHostsPath
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
