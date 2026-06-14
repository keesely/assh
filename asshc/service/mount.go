package service

import (
	"fmt"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// MountService 挂载服务编排层。
type MountService struct {
	manager port.MountManager
	repo    port.ServerRepository
	store   port.MountStore
}

// NewMountService 创建 MountService 实例。
func NewMountService(manager port.MountManager, repo port.ServerRepository, store port.MountStore) *MountService {
	return &MountService{
		manager: manager,
		repo:    repo,
		store:   store,
	}
}

// Mount 挂载远程文件系统。
func (s *MountService) Mount(serverName, remotePath, mountPoint string, opts port.MountOptions) error {
	// 获取服务器信息
	server, err := s.repo.Get(serverName)
	if err != nil {
		return fmt.Errorf("server not found: %s: %w", serverName, err)
	}

	// 设置挂载点
	opts.MountPoint = mountPoint

	// 执行挂载
	return s.manager.Mount(server, remotePath, opts)
}

// MountWithDirect 直连模式挂载（不通过服务器配置）。
func (s *MountService) MountWithDirect(host string, portNum int, user, password, keyFile, remotePath, mountPoint string, opts port.MountOptions) error {
	// 构造临时服务器对象
	server := &domain.Server{
		Host: host,
		Port: portNum,
		User: user,
		Auth: &domain.Auth{
			Password: password,
			KeyFile:  keyFile,
		},
	}

	// 设置挂载点
	opts.MountPoint = mountPoint

	// 执行挂载
	return s.manager.Mount(server, remotePath, opts)
}

// Unmount 卸载指定挂载点。
func (s *MountService) Unmount(mountPoint string) error {
	return s.manager.Unmount(mountPoint)
}

// UnmountByServer 卸载指定服务器的所有挂载。
func (s *MountService) UnmountByServer(serverName string) error {
	return s.manager.UnmountByServer(serverName)
}

// UnmountAll 卸载所有挂载。
func (s *MountService) UnmountAll() error {
	return s.manager.UnmountAll()
}

// List 列出所有挂载。
func (s *MountService) List() ([]port.MountInfo, error) {
	return s.manager.List()
}

// CheckDependencies 检查依赖状态。
func (s *MountService) CheckDependencies() map[port.MountType]port.DependencyStatus {
	return s.manager.CheckDependencies()
}
