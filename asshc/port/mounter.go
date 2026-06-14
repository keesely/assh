// Package port 定义挂载相关的接口和类型。
//
// Phase 12: 远程服务器挂载 (sshfs + WebDAV)
package port

import (
	"time"

	"assh/asshc/domain"
)

// MountType 挂载类型枚举。
type MountType string

const (
	MountTypeSSHFS  MountType = "sshfs"
	MountTypeWebDAV MountType = "webdav"
)

// MountOptions 挂载选项。
type MountOptions struct {
	MountType  MountType         // 挂载类型
	MountPoint string            // 本地挂载点（必填）
	ReadOnly   bool              // 只读模式
	AllowOther bool              // 允许其他用户访问
	VolumeName string            // 卷名称（macOS Finder 显示）
	Daemon     bool              // 后台模式
	ExtraOpts  map[string]string // 额外挂载参数
}

// MountInfo 挂载信息，用于持久化和查询。
type MountInfo struct {
	ID         int64     // 数据库 ID
	ServerName string    // 服务器名称
	RemotePath string    // 远程路径
	MountPoint string    // 本地挂载点
	MountType  MountType // 挂载类型
	LocalPort  int       // WebDAV 本地端口（仅 WebDAV）
	Daemon     bool      // 是否后台模式
	PID        int       // 后台进程 PID（仅 daemon）
	MountedAt  time.Time // 挂载时间
}

// FileMounter 单类型挂载器接口。
// 每种挂载类型（sshfs/webdav）实现此接口。
type FileMounter interface {
	// Mount 挂载远程文件系统到本地。
	Mount(server *domain.Server, remotePath string, opts MountOptions) error

	// Unmount 卸载已挂载的文件系统。
	Unmount(mountPoint string) error

	// IsAvailable 检查依赖是否满足。
	IsAvailable() bool

	// Name 返回挂载器名称。
	Name() MountType
}

// MountManager 挂载管理器接口。
// 统一管理多种挂载类型的生命周期。
type MountManager interface {
	// Mount 挂载远程文件系统。
	Mount(server *domain.Server, remotePath string, opts MountOptions) error

	// Unmount 卸载指定挂载点。
	Unmount(mountPoint string) error

	// UnmountByServer 卸载指定服务器的所有挂载。
	UnmountByServer(serverName string) error

	// UnmountAll 卸载所有挂载。
	UnmountAll() error

	// List 列出所有挂载。
	List() ([]MountInfo, error)

	// GetMounter 获取指定类型的挂载器。
	GetMounter(mountType MountType) (FileMounter, error)

	// CheckDependencies 检查依赖状态。
	CheckDependencies() map[MountType]DependencyStatus
}

// MountStore 挂载信息持久化接口。
type MountStore interface {
	// Save 保存挂载信息。
	Save(info MountInfo) error

	// DeleteByMountPoint 按挂载点删除。
	DeleteByMountPoint(mountPoint string) error

	// DeleteByServer 按服务器名删除所有挂载。
	DeleteByServer(serverName string) error

	// List 列出所有挂载信息。
	List() ([]MountInfo, error)

	// GetByMountPoint 按挂载点查询。
	GetByMountPoint(mountPoint string) (*MountInfo, error)
}

// DependencyStatus 依赖状态。
type DependencyStatus struct {
	Available bool
	Details   []DependencyItem
}

// DependencyItem 单个依赖项。
type DependencyItem struct {
	Name    string // 依赖名称
	Found   bool   // 是否找到
	Path    string // 路径（如果找到）
	Version string // 版本（如果找到）
	Install string // 安装命令（如果未找到）
}
