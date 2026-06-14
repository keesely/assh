package mount

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"assh/asshc/domain"
	"assh/asshc/port"
	"assh/log"
)

// SSHFSMounter 实现 sshfs 挂载器。
type SSHFSMounter struct{}

// NewSSHFSMounter 创建 SSHFSMounter 实例。
func NewSSHFSMounter() *SSHFSMounter {
	return &SSHFSMounter{}
}

// 编译期接口合规检查
var _ port.FileMounter = (*SSHFSMounter)(nil)

// Mount 挂载远程文件系统到本地。
func (m *SSHFSMounter) Mount(server *domain.Server, remotePath string, opts port.MountOptions) error {
	// 检查依赖
	if !m.IsAvailable() {
		return fmt.Errorf("sshfs is not installed. Install with: %s", getInstallHint("sshfs"))
	}

	// 检查挂载点
	if err := m.prepareMountPoint(opts.MountPoint); err != nil {
		return fmt.Errorf("prepare mount point: %w", err)
	}

	// 构建 sshfs 命令
	args := m.buildArgs(server, remotePath, opts)
	cmd := exec.Command("sshfs", args...)
	log.Infof("executing sshfs: %s", cmd.String())

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sshfs failed: %s: %w", string(output), err)
	}

	log.Infof("sshfs mounted successfully: %s", opts.MountPoint)
	return nil
}

// Unmount 卸载已挂载的文件系统。
func (m *SSHFSMounter) Unmount(mountPoint string) error {
	var cmd *exec.Cmd

	if runtime.GOOS == "darwin" {
		cmd = exec.Command("umount", mountPoint)
	} else {
		cmd = exec.Command("fusermount", "-u", mountPoint)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unmount failed: %s: %w", string(output), err)
	}

	log.Infof("sshfs unmounted successfully: %s", mountPoint)
	return nil
}

// IsAvailable 检查 sshfs 是否可用。
func (m *SSHFSMounter) IsAvailable() bool {
	_, err := exec.LookPath("sshfs")
	return err == nil
}

// Name 返回挂载器名称。
func (m *SSHFSMounter) Name() port.MountType {
	return port.MountTypeSSHFS
}

// buildArgs 构建 sshfs 命令行参数。
func (m *SSHFSMounter) buildArgs(server *domain.Server, remotePath string, opts port.MountOptions) []string {
	// 用户@主机:远程路径 本地路径
	user := server.User
	if user == "" {
		user = "root"
	}
	remote := fmt.Sprintf("%s@%s:%s", user, server.Host, remotePath)
	args := []string{remote, opts.MountPoint}

	// 端口
	if server.Port > 0 && server.Port != 22 {
		args = append(args, "-p", strconv.Itoa(server.Port))
	}

	// SSH 密钥
	if server.Auth != nil && server.Auth.KeyFile != "" {
		args = append(args, "-o", fmt.Sprintf("IdentityFile=%s", server.Auth.KeyFile))
	}

	// 密码认证（通过 SSH_ASKPASS）
	if server.Auth != nil && server.Auth.Password != "" {
		// 注意：密码认证需要特殊处理，这里设置环境变量
		os.Setenv("SSHPASS", server.Auth.Password)
	}

	// 只读模式
	if opts.ReadOnly {
		args = append(args, "-o", "ro")
	}

	// 允许其他用户访问
	if opts.AllowOther {
		args = append(args, "-o", "allow_other")
	}

	// 自动重连
	args = append(args, "-o", "reconnect")
	args = append(args, "-o", "ServerAliveInterval=15")

	// 卷名 (macOS Finder)
	if runtime.GOOS == "darwin" && opts.VolumeName != "" {
		args = append(args, "-o", "volname="+opts.VolumeName)
	}

	// 额外选项
	for k, v := range opts.ExtraOpts {
		if v == "" || v == "true" {
			args = append(args, "-o", k)
		} else {
			args = append(args, "-o", k+"="+v)
		}
	}

	return args
}

// prepareMountPoint 准备挂载点目录。
func (m *SSHFSMounter) prepareMountPoint(mountPoint string) error {
	// 确保是绝对路径
	if !filepath.IsAbs(mountPoint) {
		return fmt.Errorf("mount point must be an absolute path: %s", mountPoint)
	}

	// 创建目录（如果不存在）
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return fmt.Errorf("create mount point: %w", err)
	}

	// 检查是否已有挂载
	if isMounted(mountPoint) {
		return fmt.Errorf("mount point already mounted: %s", mountPoint)
	}

	return nil
}

// isMounted 检查路径是否已挂载。
func isMounted(mountPoint string) bool {
	var stat syscall.Stat_t
	if err := syscall.Stat(mountPoint, &stat); err != nil {
		return false
	}

	// 检查设备号是否与父目录不同（挂载点的特征）
	parentDir := filepath.Dir(mountPoint)
	var parentStat syscall.Stat_t
	if err := syscall.Stat(parentDir, &parentStat); err != nil {
		return false
	}

	// 如果设备号不同，说明是挂载点
	return stat.Dev != parentStat.Dev
}

// GetMountPoint 获取默认挂载点。
func GetMountPoint(serverName, remotePath string) string {
	homeDir, _ := os.UserHomeDir()
	cleanPath := strings.ReplaceAll(remotePath, "/", "_")
	if cleanPath == "_" {
		cleanPath = ""
	}
	return filepath.Join(homeDir, ".assh", "mounts", serverName+cleanPath)
}
