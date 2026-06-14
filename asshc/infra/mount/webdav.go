package mount

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"assh/asshc/domain"
	"assh/asshc/port"
	"assh/log"
)

// WebDAVMounter 实现 WebDAV 挂载器。
// 使用 Go 内置 WebDAV 服务端 + SSH 隧道实现。
type WebDAVMounter struct {
	mu       sync.Mutex
	servers  map[string]*webdavInstance // key: mountPoint
}

type webdavInstance struct {
	server   *http.Server
	listener net.Listener
	port     int
	cancel   context.CancelFunc
}

// NewWebDAVMounter 创建 WebDAVMounter 实例。
func NewWebDAVMounter() *WebDAVMounter {
	return &WebDAVMounter{
		servers: make(map[string]*webdavInstance),
	}
}

// 编译期接口合规检查
var _ port.FileMounter = (*WebDAVMounter)(nil)

// Mount 挂载远程文件系统到本地（WebDAV 模式）。
// P1 阶段：基础实现，后续需要对接 SFTP 后端。
func (m *WebDAVMounter) Mount(server *domain.Server, remotePath string, opts port.MountOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查依赖
	if !m.IsAvailable() {
		return fmt.Errorf("WebDAV mount tool not available. Install with: %s", getWebDAVInstallHint())
	}

	// 查找可用端口
	port, err := FindWebDAVPort()
	if err != nil {
		return fmt.Errorf("find available port: %w", err)
	}

	// 准备挂载点
	if err := prepareMountPoint(opts.MountPoint); err != nil {
		return fmt.Errorf("prepare mount point: %w", err)
	}

	// P1: 启动简单的 HTTP 服务端（占位，后续需要 SFTP 后端）
	ctx, cancel := context.WithCancel(context.Background())
	instance, err := m.startWebDAVServer(ctx, port, server, remotePath)
	if err != nil {
		cancel()
		return fmt.Errorf("start WebDAV server: %w", err)
	}
	instance.cancel = cancel

	// 挂载
	if err := m.mountWebDAV(port, opts.MountPoint); err != nil {
		instance.server.Shutdown(context.Background())
		return fmt.Errorf("mount WebDAV: %w", err)
	}

	// 保存实例
	m.servers[opts.MountPoint] = instance

	log.Infof("WebDAV mounted successfully: %s (port %d)", opts.MountPoint, port)
	return nil
}

// Unmount 卸载已挂载的 WebDAV 文件系统。
func (m *WebDAVMounter) Unmount(mountPoint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 卸载文件系统
	if err := unmountPath(mountPoint); err != nil {
		log.Warnf("unmount warning: %v", err)
	}

	// 停止 WebDAV 服务端
	if instance, exists := m.servers[mountPoint]; exists {
		instance.cancel()
		instance.server.Shutdown(context.Background())
		delete(m.servers, mountPoint)
	}

	log.Infof("WebDAV unmounted successfully: %s", mountPoint)
	return nil
}

// IsAvailable 检查 WebDAV 挂载工具是否可用。
func (m *WebDAVMounter) IsAvailable() bool {
	return checkWebDAVTool()
}

// Name 返回挂载器名称。
func (m *WebDAVMounter) Name() port.MountType {
	return port.MountTypeWebDAV
}

// startWebDAVServer 启动 WebDAV 服务端。
// P1 阶段：简单的占位实现，后续需要对接 SFTP 后端。
func (m *WebDAVMounter) startWebDAVServer(ctx context.Context, port int, srv *domain.Server, remotePath string) (*webdavInstance, error) {
	// 创建监听器
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}

	// P1: 简单的 HTTP 服务端（占位）
	// TODO: 对接 SFTP 后端，实现完整的 WebDAV 协议
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "WebDAV server for %s:%s (P1 stub)", srv.Name, remotePath)
	})

	httpServer := &http.Server{
		Handler: mux,
	}

	// 启动服务端
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Errorf("WebDAV server error: %v", err)
		}
	}()

	return &webdavInstance{
		server:   httpServer,
		listener: listener,
		port:     port,
	}, nil
}

// mountWebDAV 调用系统命令挂载 WebDAV。
func (m *WebDAVMounter) mountWebDAV(port int, mountPoint string) error {
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("mount_webdav", "-S", url, mountPoint)
	case "linux":
		cmd = exec.Command("mount", "-t", "davfs", url, mountPoint)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount_webdav failed: %s: %w", string(output), err)
	}

	return nil
}

// checkWebDAVTool 检查系统是否有 WebDAV 挂载工具。
func checkWebDAVTool() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := exec.LookPath("mount_webdav")
		return err == nil
	case "linux":
		_, err := exec.LookPath("mount.davfs")
		return err == nil
	default:
		return false
	}
}

// getWebDAVInstallHint 获取 WebDAV 工具安装提示。
func getWebDAVInstallHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS has built-in mount_webdav support"
	case "linux":
		return "sudo apt install davfs2  # or: sudo yum install davfs2"
	default:
		return "install WebDAV mount tool for your platform"
	}
}

// prepareMountPoint 准备挂载点目录。
func prepareMountPoint(mountPoint string) error {
	if !filepath.IsAbs(mountPoint) {
		return fmt.Errorf("mount point must be an absolute path: %s", mountPoint)
	}
	return os.MkdirAll(mountPoint, 0755)
}

// unmountPath 卸载指定路径。
func unmountPath(mountPoint string) error {
	cmd := exec.Command("umount", mountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("umount failed: %s: %w", string(output), err)
	}
	return nil
}
