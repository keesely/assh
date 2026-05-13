// Package proxy 提供 SOCKS5 代理和端口转发的基础设施实现。
//
// 实现 port.Proxy 和 port.PortForward 接口。
// SOCKS5 实现遵循 RFC 1928，支持无认证 CONNECT 模式。
package proxy

import (
	"fmt"
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"

	"assh/log"
)

// SOCKS5 协议常量（RFC 1928）
const (
	socksVer5          = 5
	socksAuthNone      = 0
	socksAuthUserPass  = 2
	socksCmdConnect    = 1
	socksATYPIPv4      = 1
	socksATYPDomain    = 3
	socksATYPIPv6      = 4
	socksRepSuccess    = 0
	socksRepFailure    = 1
	socksRepNotAllowed = 2
	socksRepUnreachable = 3
	socksRepCmdUnsup   = 7
	socksRepATYPUnsup  = 8
)

// SOCKS5Proxy 实现 port.Proxy 接口的 SOCKS5 代理。
// 监听本地端口，通过 SSH 隧道转发 SOCKS5 客户端请求。
type SOCKS5Proxy struct {
	mu       sync.Mutex
	listener net.Listener
	addr     string
	active   map[net.Conn]struct{}
	authFunc func(username, password string) bool
}

// NewSOCKS5Proxy 创建 SOCKS5 代理实例（无认证模式）。
func NewSOCKS5Proxy() *SOCKS5Proxy {
	return &SOCKS5Proxy{
		active: make(map[net.Conn]struct{}),
	}
}

// NewSOCKS5ProxyWithAuth 创建带 USERNAME/PASSWORD 认证的 SOCKS5 代理实例。
// authFn 接收用户名和密码，返回是否认证通过。为 nil 时等同于无认证模式。
func NewSOCKS5ProxyWithAuth(authFn func(username, password string) bool) *SOCKS5Proxy {
	return &SOCKS5Proxy{
		active:   make(map[net.Conn]struct{}),
		authFunc: authFn,
	}
}

// Start 在指定地址启动 SOCKS5 代理服务。
// localAddr 格式为 "host:port"（如 "127.0.0.1:1080"）。
func (p *SOCKS5Proxy) Start(client *ssh.Client, localAddr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.listener != nil {
		return fmt.Errorf("SOCKS5 proxy already running on %s", p.addr)
	}

	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", localAddr, err)
	}

	p.listener = listener
	p.addr = listener.Addr().String()

	log.Infof("SOCKS5 proxy started on %s (via SSH)", p.addr)

	go p.acceptLoop(client)
	return nil
}

// Stop 停止 SOCKS5 代理，关闭监听器和所有活跃连接。
func (p *SOCKS5Proxy) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.listener == nil {
		return nil
	}

	// 关闭监听器
	if err := p.listener.Close(); err != nil {
		log.Warnf("close SOCKS5 listener: %v", err)
	}

	// 关闭所有活跃连接
	for conn := range p.active {
		conn.Close()
	}

	p.listener = nil
	p.addr = ""
	p.active = make(map[net.Conn]struct{})
	log.Infof("SOCKS5 proxy stopped")
	return nil
}

// Addr 返回代理的监听地址。
func (p *SOCKS5Proxy) Addr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.addr
}

// acceptLoop 循环接受客户端连接。
func (p *SOCKS5Proxy) acceptLoop(client *ssh.Client) {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			// 监听器已关闭（正常停止）
			return
		}

		p.mu.Lock()
		p.active[conn] = struct{}{}
		p.mu.Unlock()

		go func() {
			p.handleConnection(client, conn)
			p.mu.Lock()
			delete(p.active, conn)
			p.mu.Unlock()
		}()
	}
}

// handleConnection 处理单个 SOCKS5 客户端连接。
// 执行协议握手，然后转发数据。
func (p *SOCKS5Proxy) handleConnection(client *ssh.Client, conn net.Conn) {
	defer conn.Close()

	// 1. 握手：读取客户端认证方法协商
	if err := p.socksHandshake(conn); err != nil {
		log.Debugf("SOCKS5 handshake failed: %v", err)
		return
	}

	// 2. 读取客户端请求
	targetAddr, err := p.readRequest(conn)
	if err != nil {
		log.Debugf("SOCKS5 request failed: %v", err)
		return
	}

	// 3. 通过 SSH 连接到目标
	remoteConn, err := client.Dial("tcp", targetAddr)
	if err != nil {
		log.Debugf("SOCKS5 connect to %s via SSH: %v", targetAddr, err)
		p.sendReply(conn, socksRepUnreachable)
		return
	}
	defer remoteConn.Close()

	// 4. 发送成功响应
	if err := p.sendReply(conn, socksRepSuccess); err != nil {
		return
	}

	// 5. 双向转发数据
	log.Debugf("SOCKS5 proxy: forwarding %s <-> %s", conn.RemoteAddr(), targetAddr)
	bidirectionalCopy(conn, remoteConn)
}

// socksHandshake 执行 SOCKS5 握手，支持 RFC 1928 无认证和 RFC 1929 用户名密码认证。
// 格式: Client → | ver | nmethods | methods... |
//
//	Server → | ver | method |
//
// 当 authFunc 不为 nil 时，优先选择 USERNAME/PASSWORD (0x02) 认证方式。
func (p *SOCKS5Proxy) socksHandshake(conn net.Conn) error {
	// 读取客户端握手
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("read handshake: %w", err)
	}

	if header[0] != socksVer5 {
		return fmt.Errorf("unsupported SOCKS version: %d", header[0])
	}

	nmethods := int(header[1])
	if nmethods < 1 || nmethods > 255 {
		return fmt.Errorf("invalid nmethods: %d", nmethods)
	}

	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return fmt.Errorf("read methods: %w", err)
	}

	// 确定支持的认证方法
	supportsNoAuth := false
	supportsUserPass := false
	for _, m := range methods {
		switch m {
		case socksAuthNone:
			supportsNoAuth = true
		case socksAuthUserPass:
			supportsUserPass = true
		}
	}

	// 选择认证方法：USERNAME/PASSWORD（0x02）优先于 NO AUTH（0x00）
	switch {
	case p.authFunc != nil && supportsUserPass:
		// 选择 USERNAME/PASSWORD 认证
		if _, err := conn.Write([]byte{socksVer5, socksAuthUserPass}); err != nil {
			return fmt.Errorf("write handshake response: %w", err)
		}
		if err := p.authUserPass(conn); err != nil {
			return fmt.Errorf("auth failed: %w", err)
		}
		return nil

	case supportsNoAuth:
		// 选择无认证
		if _, err := conn.Write([]byte{socksVer5, socksAuthNone}); err != nil {
			return fmt.Errorf("write handshake response: %w", err)
		}
		return nil

	default:
		// 无匹配的认证方法
		conn.Write([]byte{socksVer5, 0xFF})
		return fmt.Errorf("no acceptable auth method")
	}
}

// authUserPass 执行 RFC 1929 USERNAME/PASSWORD 认证子协商。
// 格式: Client → | ver=1 | ulen | uname | plen | passwd |
//
//	Server → | ver=1 | status=0 |
func (p *SOCKS5Proxy) authUserPass(conn net.Conn) error {
	// 读取版本和用户名长度
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("read auth header: %w", err)
	}
	if header[0] != 1 {
		return fmt.Errorf("unsupported auth version: %d", header[0])
	}

	// 读取用户名
	uname := make([]byte, header[1])
	if _, err := io.ReadFull(conn, uname); err != nil {
		return fmt.Errorf("read username: %w", err)
	}

	// 读取密码长度
	plenBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, plenBuf); err != nil {
		return fmt.Errorf("read password length: %w", err)
	}

	// 读取密码
	passwd := make([]byte, plenBuf[0])
	if _, err := io.ReadFull(conn, passwd); err != nil {
		return fmt.Errorf("read password: %w", err)
	}

	// 验证凭据
	if p.authFunc(string(uname), string(passwd)) {
		if _, err := conn.Write([]byte{1, 0}); err != nil {
			return fmt.Errorf("write auth success: %w", err)
		}
		log.Debugf("SOCKS5 auth success: user=%q", string(uname))
		return nil
	}

	// 认证失败
	conn.Write([]byte{1, 1})
	log.Debugf("SOCKS5 auth failed: user=%q", string(uname))
	return fmt.Errorf("SOCKS5 auth failed for user %q", string(uname))
}

// readRequest 读取 SOCKS5 客户端请求。
// 格式: | ver=5 | cmd=1 | rsv=0 | atyp | dst.addr | dst.port |
// 返回目标地址字符串 "host:port"。
func (p *SOCKS5Proxy) readRequest(conn net.Conn) (string, error) {
	// 读取固定头部
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", fmt.Errorf("read request header: %w", err)
	}

	if header[0] != socksVer5 {
		return "", fmt.Errorf("unsupported version in request: %d", header[0])
	}

	if header[1] != socksCmdConnect {
		// 不支持的命令
		p.sendReply(conn, socksRepCmdUnsup)
		return "", fmt.Errorf("unsupported command: %d", header[1])
	}

	// 读取目标地址
	var host string
	switch header[3] {
	case socksATYPIPv4:
		// IPv4: 4 字节
		addr := make([]byte, 4)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", fmt.Errorf("read IPv4: %w", err)
		}
		host = net.IP(addr).String()

	case socksATYPDomain:
		// 域名: 1 字节长度 + 域名
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", fmt.Errorf("read domain length: %w", err)
		}
		domain := make([]byte, lenBuf[0])
		if _, err := io.ReadFull(conn, domain); err != nil {
			return "", fmt.Errorf("read domain: %w", err)
		}
		host = string(domain)

	case socksATYPIPv6:
		// IPv6: 16 字节
		addr := make([]byte, 16)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", fmt.Errorf("read IPv6: %w", err)
		}
		host = net.IP(addr).String()

	default:
		p.sendReply(conn, socksRepATYPUnsup)
		return "", fmt.Errorf("unsupported address type: %d", header[3])
	}

	// 读取端口（2 字节，大端序）
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", fmt.Errorf("read port: %w", err)
	}
	port := int(portBuf[0])<<8 | int(portBuf[1])

	return fmt.Sprintf("%s:%d", host, port), nil
}

// sendReply 发送 SOCKS5 响应。
// 格式: | ver=5 | rep | rsv=0 | atyp | bnd.addr | bnd.port |
func (p *SOCKS5Proxy) sendReply(conn net.Conn, rep byte) error {
	// BND.ADDR 和 BND.PORT 可设为 0（客户端通常忽略）
	reply := []byte{
		socksVer5,
		rep,
		0x00, // RSV
		socksATYPIPv4,
		0x00, 0x00, 0x00, 0x00, // BND.ADDR = 0.0.0.0
		0x00, 0x00, // BND.PORT = 0
	}
	_, err := conn.Write(reply)
	return err
}

// bidirectionalCopy 在两个连接之间双向复制数据。
// 任一方向完成或出错时，关闭两个连接。
func bidirectionalCopy(c1, c2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if _, err := io.Copy(c1, c2); err != nil {
			// 预期内的 EOF 错误忽略
			if opErr, ok := err.(*net.OpError); !ok || opErr.Err.Error() != "use of closed network connection" {
				log.Debugf("bidirectional copy c1<-c2: %v", err)
			}
		}
		c1.Close()
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(c2, c1); err != nil {
			if opErr, ok := err.(*net.OpError); !ok || opErr.Err.Error() != "use of closed network connection" {
				log.Debugf("bidirectional copy c2<-c1: %v", err)
			}
		}
		c2.Close()
	}()

	wg.Wait()
}
