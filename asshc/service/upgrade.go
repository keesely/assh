package service

import (
	"context"
	"fmt"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// UpgradeService 编排层，负责自动更新/升级的业务逻辑。
type UpgradeService struct {
	checker  port.VersionChecker
	updater  port.Updater
}

// NewUpgradeService 创建新的升级服务。
// 参数：
//   - checker: 版本检查器
//   - updater: 更新器
func NewUpgradeService(checker port.VersionChecker, updater port.Updater) *UpgradeService {
	return &UpgradeService{
		checker: checker,
		updater: updater,
	}
}

// CheckUpdate 检查是否有新版本可用。
// 返回：
//   - *domain.UpdateInfo: 更新信息（如果有新版本）
//   - error: 检查失败时返回错误
func (s *UpgradeService) CheckUpdate() (*domain.UpdateInfo, error) {
	return s.checker.CheckUpdate()
}

// GetCurrentVersion 获取当前版本号。
func (s *UpgradeService) GetCurrentVersion() string {
	return s.checker.GetCurrentVersion()
}

// GetPlatform 获取当前平台信息。
func (s *UpgradeService) GetPlatform() domain.UpdatePlatform {
	return s.checker.GetPlatform()
}

// UpgradeOptions 定义升级选项。
type UpgradeOptions struct {
	Version string // 指定版本号（为空则升级到最新）
	Force   bool   // 强制升级（忽略版本检查）
	Yes     bool   // 自动确认
}

// Upgrade 执行升级操作。
//
// 流程：
//  1. 检查更新（除非 Force 为 true）
//  2. 下载新版本
//  3. 验证下载文件
//  4. 安装更新
//
// 参数：
//   - ctx: 上下文
//   - options: 升级选项
//   - progress: 下载进度回调
//
// 返回：
//   - *domain.UpdateInfo: 更新信息
//   - error: 升级失败时返回错误
func (s *UpgradeService) Upgrade(ctx context.Context, options UpgradeOptions, progress port.DownloadProgress) (*domain.UpdateInfo, error) {
	var updateInfo *domain.UpdateInfo
	var err error

	// 1. 检查更新
	if options.Force && options.Version != "" {
		// 强制模式：创建指定版本的 UpdateInfo
		updateInfo = &domain.UpdateInfo{
			Version: options.Version,
		}
	} else {
		// 正常模式：检查更新
		updateInfo, err = s.checker.CheckUpdate()
		if err != nil {
			return nil, err
		}

		// 如果指定了版本，检查是否匹配
		if options.Version != "" && updateInfo.Version != options.Version {
			return nil, fmt.Errorf("requested version %s not available, latest is %s", options.Version, updateInfo.Version)
		}
	}

	// 2. 下载新版本
	downloadPath, err := s.updater.Download(ctx, updateInfo, progress)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// 3. 验证下载文件
	if updateInfo.SHA256 != "" {
		if err := s.updater.Verify(downloadPath, updateInfo.SHA256); err != nil {
			return nil, err
		}
	}

	// 4. 安装更新
	if err := s.updater.Install(ctx, downloadPath); err != nil {
		return nil, fmt.Errorf("install failed: %w", err)
	}

	return updateInfo, nil
}

// Rollback 回滚到上一版本。
func (s *UpgradeService) Rollback() error {
	return s.updater.Rollback()
}

// GetBackupDir 获取备份目录路径。
func (s *UpgradeService) GetBackupDir() string {
	return s.updater.GetBackupDir()
}
