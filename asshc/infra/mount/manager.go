package mount

import (
	"fmt"
	"sync"

	"assh/asshc/domain"
	"assh/asshc/port"
	"assh/log"
)

// MountManagerImpl 实现 port.MountManager 接口。
type MountManagerImpl struct {
	mounters map[port.MountType]port.FileMounter
	store    port.MountStore
	mu       sync.RWMutex
}

// NewMountManagerImpl 创建 MountManagerImpl 实例。
func NewMountManagerImpl(store port.MountStore) *MountManagerImpl {
	mm := &MountManagerImpl{
		mounters: make(map[port.MountType]port.FileMounter),
		store:    store,
	}

	// 注册默认挂载器
	mm.registerDefaultMounters()

	return mm
}

// registerDefaultMounters 注册默认的挂载器。
func (mm *MountManagerImpl) registerDefaultMounters() {
	mm.mounters[port.MountTypeSSHFS] = NewSSHFSMounter()
	// WebDAV 挂载器将在 P12.10 实现后注册
}

// RegisterMounter 注册自定义挂载器。
func (mm *MountManagerImpl) RegisterMounter(mounter port.FileMounter) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.mounters[mounter.Name()] = mounter
}

// Mount 挂载远程文件系统。
func (mm *MountManagerImpl) Mount(server *domain.Server, remotePath string, opts port.MountOptions) error {
	mm.mu.RLock()
	mounter, exists := mm.mounters[opts.MountType]
	mm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("unsupported mount type: %s", opts.MountType)
	}

	// 执行挂载
	if err := mounter.Mount(server, remotePath, opts); err != nil {
		return err
	}

	// 保存挂载信息
	info := port.MountInfo{
		ServerName: server.Name,
		RemotePath: remotePath,
		MountPoint: opts.MountPoint,
		MountType:  opts.MountType,
		Daemon:     opts.Daemon,
	}

	if err := mm.store.Save(info); err != nil {
		log.Errorf("failed to save mount info: %v", err)
		// 尝试卸载
		if unmountErr := mounter.Unmount(opts.MountPoint); unmountErr != nil {
			log.Errorf("failed to unmount after save error: %v", unmountErr)
		}
		return fmt.Errorf("save mount info: %w", err)
	}

	return nil
}

// Unmount 卸载指定挂载点。
func (mm *MountManagerImpl) Unmount(mountPoint string) error {
	// 获取挂载信息
	info, err := mm.store.GetByMountPoint(mountPoint)
	if err != nil {
		return fmt.Errorf("mount point not found: %s: %w", mountPoint, err)
	}

	// 获取对应的挂载器
	mm.mu.RLock()
	mounter, exists := mm.mounters[info.MountType]
	mm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("unsupported mount type: %s", info.MountType)
	}

	// 执行卸载
	if err := mounter.Unmount(mountPoint); err != nil {
		return err
	}

	// 删除挂载记录
	return mm.store.DeleteByMountPoint(mountPoint)
}

// UnmountByServer 卸载指定服务器的所有挂载。
func (mm *MountManagerImpl) UnmountByServer(serverName string) error {
	mounts, err := mm.store.List()
	if err != nil {
		return fmt.Errorf("list mounts: %w", err)
	}

	var lastErr error
	for _, info := range mounts {
		if info.ServerName == serverName {
			if err := mm.Unmount(info.MountPoint); err != nil {
				log.Errorf("failed to unmount %s: %v", info.MountPoint, err)
				lastErr = err
			}
		}
	}

	return lastErr
}

// UnmountAll 卸载所有挂载。
func (mm *MountManagerImpl) UnmountAll() error {
	mounts, err := mm.store.List()
	if err != nil {
		return fmt.Errorf("list mounts: %w", err)
	}

	var lastErr error
	for _, info := range mounts {
		if err := mm.Unmount(info.MountPoint); err != nil {
			log.Errorf("failed to unmount %s: %v", info.MountPoint, err)
			lastErr = err
		}
	}

	return lastErr
}

// List 列出所有挂载。
func (mm *MountManagerImpl) List() ([]port.MountInfo, error) {
	return mm.store.List()
}

// GetMounter 获取指定类型的挂载器。
func (mm *MountManagerImpl) GetMounter(mountType port.MountType) (port.FileMounter, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	mounter, exists := mm.mounters[mountType]
	if !exists {
		return nil, fmt.Errorf("unsupported mount type: %s", mountType)
	}
	return mounter, nil
}

// CheckDependencies 检查依赖状态。
func (mm *MountManagerImpl) CheckDependencies() map[port.MountType]port.DependencyStatus {
	return CheckDependencies()
}
