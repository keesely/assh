package mount

import (
	"fmt"
	"net"
)

// WebDAV 端口范围（避免与常用端口冲突）
const (
	WebDAVPortMin = 18000
	WebDAVPortMax = 18999
)

// FindAvailablePort 在指定范围内查找可用端口。
// 如果 min/max 为 0，则使用随机端口。
func FindAvailablePort(min, max int) (int, error) {
	if min == 0 || max == 0 {
		// 使用随机端口
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, fmt.Errorf("find random port: %w", err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		return port, nil
	}

	// 在指定范围内查找
	for port := min; port <= max; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available port in range %d-%d", min, max)
}

// IsPortAvailable 检查指定端口是否可用。
func IsPortAvailable(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// FindWebDAVPort 查找可用的 WebDAV 端口（18000-18999 范围）。
func FindWebDAVPort() (int, error) {
	return FindAvailablePort(WebDAVPortMin, WebDAVPortMax)
}
