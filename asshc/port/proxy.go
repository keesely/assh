// Package port 定义代理和隧道功能的业务接口（端口）。
//
// 本包只定义接口和领域类型引用，不包含任何实现。
// 提供 SOCKS5 代理、端口转发和隧道管理三类接口。
package port

import "golang.org/x/crypto/ssh"

// Proxy 定义 SOCKS5 代理接口。
// 实现 RFC 1928 标准的 SOCKS5 协议，支持无认证 CONNECT 模式。
// Start 创建 SSH 隧道代理本地端口，Stop 停止代理并释放资源。
type Proxy interface {
	// Start 在指定地址启动 SOCKS5 代理服务。
	// localAddr 格式为 "host:port"（如 "127.0.0.1:1080"）。
	// 使用给定的 SSH 客户端建立数据转发通道。
	Start(client *ssh.Client, localAddr string) error

	// Stop 停止代理服务，关闭监听器和所有活跃连接。
	Stop() error

	// Addr 返回代理的监听地址（可能含实际绑定端口）。
	Addr() string
}

// PortForward 定义端口转发接口。
// 支持本地转发（-L）和远程转发（-R）两种模式。
type PortForward interface {
	// ID 返回转发的唯一标识符。
	ID() string

	// Type 返回转发类型："local" 或 "remote"。
	Type() string

	// LocalAddr 返回本地监听或目标地址。
	LocalAddr() string

	// RemoteAddr 返回远程监听或目标地址。
	RemoteAddr() string

	// Start 启动端口转发，使用给定的 SSH 客户端建立通道。
	Start(client *ssh.Client) error

	// Stop 停止端口转发，关闭监听器和所有活跃连接。
	Stop() error
}

// TunnelManager 定义隧道管理器接口。
// 管理多个端口转发的生命周期，支持添加、移除、查询和批量启停。
type TunnelManager interface {
	// Add 添加一个端口转发到管理器。
	Add(forward PortForward) error

	// Remove 移除并停止指定 ID 的端口转发。
	Remove(id string) error

	// Get 根据 ID 获取端口转发。
	Get(id string) (PortForward, error)

	// List 返回所有已添加的端口转发。
	List() []PortForward

	// StopAll 停止所有端口转发。
	StopAll() error
}
