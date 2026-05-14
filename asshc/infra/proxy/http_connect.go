package proxy

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"

	"assh/log"
)

// HTTPConnectProxy 实现 HTTP CONNECT 代理（RFC 7231 §4.3.6）。
// 监听本地端口，接收 HTTP CONNECT 请求，通过 SSH 隧道转发到目标。
type HTTPConnectProxy struct {
	mu       sync.Mutex
	listener net.Listener
	addr     string
	active   map[net.Conn]struct{}
	authFunc func(username, password string) bool
}

// NewHTTPConnectProxy 创建 HTTP CONNECT 代理实例（无认证模式）。
func NewHTTPConnectProxy() *HTTPConnectProxy {
	return &HTTPConnectProxy{
		active: make(map[net.Conn]struct{}),
	}
}

// NewHTTPConnectProxyWithAuth 创建带 HTTP Basic 认证的 HTTP CONNECT 代理实例。
// authFn 接收用户名和密码，返回是否认证通过。为 nil 时等同于无认证模式。
func NewHTTPConnectProxyWithAuth(authFn func(username, password string) bool) *HTTPConnectProxy {
	return &HTTPConnectProxy{
		active:   make(map[net.Conn]struct{}),
		authFunc: authFn,
	}
}

// Start 在指定地址启动 HTTP CONNECT 代理服务。
// localAddr 格式为 "host:port"（如 "127.0.0.1:3128"）。
func (p *HTTPConnectProxy) Start(client *ssh.Client, localAddr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.listener != nil {
		return fmt.Errorf("HTTP CONNECT proxy already running on %s", p.addr)
	}

	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", localAddr, err)
	}

	p.listener = listener
	p.addr = listener.Addr().String()

	log.Infof("HTTP CONNECT proxy started on %s (via SSH)", p.addr)

	go p.acceptLoop(client)
	return nil
}

// Stop 停止 HTTP CONNECT 代理，关闭监听器和所有活跃连接。
func (p *HTTPConnectProxy) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.listener == nil {
		return nil
	}

	if err := p.listener.Close(); err != nil {
		log.Warnf("close HTTP CONNECT listener: %v", err)
	}

	for conn := range p.active {
		conn.Close()
	}

	p.listener = nil
	p.addr = ""
	p.active = make(map[net.Conn]struct{})
	log.Infof("HTTP CONNECT proxy stopped")
	return nil
}

// Addr 返回代理的监听地址。
func (p *HTTPConnectProxy) Addr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.addr
}

// acceptLoop 循环接受客户端连接。
func (p *HTTPConnectProxy) acceptLoop(client *ssh.Client) {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
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

// handleConnection 处理单个 HTTP CONNECT 客户端连接。
// 执行 HTTP 握手，然后通过 SSH 隧道转发数据。
func (p *HTTPConnectProxy) handleConnection(client *ssh.Client, conn net.Conn) {
	defer conn.Close()

	targetAddr, err := p.httpHandshake(conn)
	if err != nil {
		log.Debugf("HTTP CONNECT handshake failed: %v", err)
		return
	}

	// 通过 SSH 连接到目标
	remoteConn, err := client.Dial("tcp", targetAddr)
	if err != nil {
		log.Debugf("HTTP CONNECT dial %s via SSH: %v", targetAddr, err)
		p.writeError(conn, "502", "Bad Gateway")
		return
	}
	defer remoteConn.Close()

	// 发送成功响应
	if err := p.writeResponse(conn, "200", "Connection Established"); err != nil {
		return
	}

	// 双向转发数据
	log.Debugf("HTTP CONNECT proxy: forwarding %s <-> %s", conn.RemoteAddr(), targetAddr)
	bidirectionalCopy(conn, remoteConn)
}

// httpHandshake 执行 HTTP CONNECT 握手。
// 解析请求行，读取头部，可选地执行 Basic 认证。
// 返回目标地址 "host:port"。
func (p *HTTPConnectProxy) httpHandshake(conn net.Conn) (string, error) {
	br := bufio.NewReader(conn)

	// 读取请求行
	reqLine, err := br.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read request line: %w", err)
	}
	reqLine = strings.TrimRight(reqLine, "\r\n")

	// 解析请求行: CONNECT host:port HTTP/1.x
	parts := strings.SplitN(reqLine, " ", 3)
	if len(parts) != 3 {
		p.writeError(conn, "400", "Bad Request")
		return "", fmt.Errorf("malformed request line: %q", reqLine)
	}

	method, target, version := parts[0], parts[1], parts[2]

	if method != "CONNECT" {
		p.writeError(conn, "405", "Method Not Allowed")
		return "", fmt.Errorf("unsupported method: %s", method)
	}

	if !strings.HasPrefix(version, "HTTP/1.") {
		p.writeError(conn, "400", "Bad Request")
		return "", fmt.Errorf("unsupported HTTP version: %s", version)
	}

	// 验证目标地址格式: host:port
	if _, _, err := net.SplitHostPort(target); err != nil {
		p.writeError(conn, "400", "Bad Request")
		return "", fmt.Errorf("invalid target %q: %w", target, err)
	}

	// 读取头部直到空行
	var proxyAuth string
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}

		if p.authFunc != nil {
			if key, value, found := strings.Cut(line, ":"); found {
				if strings.EqualFold(strings.TrimSpace(key), "proxy-authorization") {
					proxyAuth = strings.TrimSpace(value)
				}
			}
		}
	}

	// 如果配置了认证，验证 Proxy-Authorization
	if p.authFunc != nil {
		if err := p.authenticate(proxyAuth); err != nil {
			p.writeAuthRequired(conn)
			return "", err
		}
	}

	return target, nil
}

// authenticate 验证 HTTP Basic 认证凭据。
func (p *HTTPConnectProxy) authenticate(authHeader string) error {
	if authHeader == "" {
		return fmt.Errorf("missing proxy authorization")
	}

	if !strings.HasPrefix(authHeader, "Basic ") {
		return fmt.Errorf("unsupported auth scheme")
	}

	encoded := strings.TrimPrefix(authHeader, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("decode base64: %w", err)
	}

	username, password, found := strings.Cut(string(decoded), ":")
	if !found {
		return fmt.Errorf("invalid credential format")
	}

	if !p.authFunc(username, password) {
		log.Debugf("HTTP CONNECT auth failed: user=%q", username)
		return fmt.Errorf("auth failed for user %q", username)
	}

	log.Debugf("HTTP CONNECT auth success: user=%q", username)
	return nil
}

// writeResponse 向客户端写入 HTTP 成功响应。
func (p *HTTPConnectProxy) writeResponse(conn net.Conn, statusCode, statusText string) error {
	_, err := fmt.Fprintf(conn, "HTTP/1.1 %s %s\r\n\r\n", statusCode, statusText)
	return err
}

// writeError 向客户端写入 HTTP 错误响应。
func (p *HTTPConnectProxy) writeError(conn net.Conn, statusCode, statusText string) {
	fmt.Fprintf(conn, "HTTP/1.1 %s %s\r\n\r\n", statusCode, statusText)
}

// writeAuthRequired 向客户端写入 407 Proxy Authentication Required 响应。
func (p *HTTPConnectProxy) writeAuthRequired(conn net.Conn) {
	fmt.Fprintf(conn, "HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"Proxy\"\r\n\r\n")
}
