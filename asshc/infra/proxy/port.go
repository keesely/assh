package proxy

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"assh/log"
)

// LocalForward 实现本地端口转发（等价于 ssh -L）。
// 在本地监听端口，将传入连接通过 SSH 转发到远程目标。
type LocalForward struct {
	id         string
	localAddr  string
	remoteAddr string
	listener   net.Listener
	active     map[net.Conn]struct{}
	mu         sync.Mutex
	done       chan struct{}
}

// NewLocalForward 创建本地端口转发实例。
// localAddr: 本地监听地址（如 "127.0.0.1:8080"）
// remoteAddr: 远程目标地址（如 "internal-host:80"）
func NewLocalForward(localAddr, remoteAddr string) *LocalForward {
	return &LocalForward{
		id:         generateID(),
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
		active:     make(map[net.Conn]struct{}),
		done:       make(chan struct{}),
	}
}

// ID 返回转发的唯一标识符。
func (f *LocalForward) ID() string { return f.id }

// Type 返回转发类型（"local"）。
func (f *LocalForward) Type() string { return "local" }

// LocalAddr 返回本地监听地址。
func (f *LocalForward) LocalAddr() string { return f.localAddr }

// RemoteAddr 返回远程目标地址。
func (f *LocalForward) RemoteAddr() string { return f.remoteAddr }

// Start 启动本地端口转发。
// 在 localAddr 上监听，新连接通过 SSH 转发到 remoteAddr。
func (f *LocalForward) Start(client *ssh.Client) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.listener != nil {
		return fmt.Errorf("local forward %s already running", f.id)
	}

	listener, err := net.Listen("tcp", f.localAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", f.localAddr, err)
	}

	f.listener = listener
	log.Debugf("local forward [%s] started: %s -> (SSH) -> %s", f.id, f.localAddr, f.remoteAddr)

	go f.acceptLoop(client)
	return nil
}

// Stop 停止本地端口转发。
func (f *LocalForward) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.listener == nil {
		return nil
	}

	if err := f.listener.Close(); err != nil {
		log.Warnf("close local forward listener: %v", err)
	}

	for conn := range f.active {
		conn.Close()
	}

	f.listener = nil
	f.active = make(map[net.Conn]struct{})
	close(f.done)
	log.Debugf("local forward [%s] stopped", f.id)
	return nil
}

func (f *LocalForward) acceptLoop(client *ssh.Client) {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}

		f.mu.Lock()
		f.active[conn] = struct{}{}
		f.mu.Unlock()

		go func() {
			f.handleConn(client, conn)
			f.mu.Lock()
			delete(f.active, conn)
			f.mu.Unlock()
		}()
	}
}

func (f *LocalForward) handleConn(client *ssh.Client, localConn net.Conn) {
	defer localConn.Close()

	remoteConn, err := client.Dial("tcp", f.remoteAddr)
	if err != nil {
		log.Debugf("local forward [%s] dial remote %s: %v", f.id, f.remoteAddr, err)
		return
	}
	defer remoteConn.Close()

	log.Debugf("local forward [%s]: forwarding %s <-> %s", f.id, localConn.RemoteAddr(), f.remoteAddr)
	bidirectionalCopy(localConn, remoteConn)
}

// RemoteForward 实现远程端口转发（等价于 ssh -R）。
// 请求 SSH 服务器在远程地址监听，将传入连接转发到本地服务。
type RemoteForward struct {
	id         string
	remoteAddr string // SSH 服务器端监听地址
	localAddr  string // 本地服务地址
	listener   net.Listener
	active     map[net.Conn]struct{}
	mu         sync.Mutex
	done       chan struct{}
}

// NewRemoteForward 创建远程端口转发实例。
// remoteAddr: SSH 服务器端监听地址（如 "0.0.0.0:8080"）
// localAddr: 本地服务地址（如 "localhost:3000"）
func NewRemoteForward(remoteAddr, localAddr string) *RemoteForward {
	return &RemoteForward{
		id:         generateID(),
		remoteAddr: remoteAddr,
		localAddr:  localAddr,
		active:     make(map[net.Conn]struct{}),
		done:       make(chan struct{}),
	}
}

// ID 返回转发的唯一标识符。
func (f *RemoteForward) ID() string { return f.id }

// Type 返回转发类型（"remote"）。
func (f *RemoteForward) Type() string { return "remote" }

// LocalAddr 返回本地目标地址。
func (f *RemoteForward) LocalAddr() string { return f.localAddr }

// RemoteAddr 返回远程监听地址。
func (f *RemoteForward) RemoteAddr() string { return f.remoteAddr }

// Start 启动远程端口转发。
// 请求 SSH 服务器在 remoteAddr 上监听，连接通过 SSH 转发到本地 localAddr。
func (f *RemoteForward) Start(client *ssh.Client) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.listener != nil {
		return fmt.Errorf("remote forward %s already running", f.id)
	}

	// ssh.Client.Listen 发送 "tcpip-forward" 全局请求，
	// 使 SSH 服务器在 remoteAddr 上监听，并通过 forwarded-tcpip 通道转发连接。
	listener, err := client.Listen("tcp", f.remoteAddr)
	if err != nil {
		return fmt.Errorf("remote listen on %s: %w", f.remoteAddr, err)
	}

	f.listener = listener
	log.Debugf("remote forward [%s] started: %s <- (SSH) <- %s", f.id, f.remoteAddr, f.localAddr)

	go f.acceptLoop(client)
	return nil
}

// Stop 停止远程端口转发。
func (f *RemoteForward) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.listener == nil {
		return nil
	}

	if err := f.listener.Close(); err != nil {
		log.Warnf("close remote forward listener: %v", err)
	}

	for conn := range f.active {
		conn.Close()
	}

	f.listener = nil
	f.active = make(map[net.Conn]struct{})
	close(f.done)
	log.Debugf("remote forward [%s] stopped", f.id)
	return nil
}

func (f *RemoteForward) acceptLoop(client *ssh.Client) {
	for {
		// listener.Accept 返回的 net.Conn 实际上是通过 SSH
		// forwarded-tcpip 通道传输数据的连接
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}

		f.mu.Lock()
		f.active[conn] = struct{}{}
		f.mu.Unlock()

		go func() {
			f.handleConn(conn)
			f.mu.Lock()
			delete(f.active, conn)
			f.mu.Unlock()
		}()
	}
}

func (f *RemoteForward) handleConn(remoteConn net.Conn) {
	defer remoteConn.Close()

	localConn, err := net.Dial("tcp", f.localAddr)
	if err != nil {
		log.Debugf("remote forward [%s] dial local %s: %v", f.id, f.localAddr, err)
		return
	}
	defer localConn.Close()

	log.Debugf("remote forward [%s]: forwarding %s <-> %s", f.id, f.remoteAddr, f.localAddr)
	bidirectionalCopy(localConn, remoteConn)
}

// generateID 生成一个随机的 8 字符十六进制 ID。
func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// 极低概率 fallback: use timestamp + PID
		return fmt.Sprintf("tunnel-%d-%d", time.Now().UnixNano(), os.Getpid())
	}
	return fmt.Sprintf("%x", b)
}
