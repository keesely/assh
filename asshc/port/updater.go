package port

import (
	"context"

	"assh/asshc/domain"
)

// DownloadProgress 定义下载进度回调函数。
// 参数：
//   - downloaded: 已下载字节数
//   - total: 总字节数
//   - percent: 下载百分比 (0.0 ~ 1.0)
type DownloadProgress func(downloaded, total int64, percent float64)

// VersionChecker 定义版本检查接口。
type VersionChecker interface {
	// CheckUpdate 检查是否有新版本可用。
	// 返回：
	//   - *domain.UpdateInfo: 更新信息（如果有新版本）
	//   - error: 检查失败时返回错误
	CheckUpdate() (*domain.UpdateInfo, error)

	// GetCurrentVersion 获取当前版本号。
	GetCurrentVersion() string

	// GetPlatform 获取当前平台信息（OS/Arch）。
	GetPlatform() domain.UpdatePlatform
}

// Updater 定义更新操作接口。
type Updater interface {
	// Download 下载指定版本的更新包。
	// 参数：
	//   - ctx: 上下文，用于控制超时和取消
	//   - info: 更新信息
	//   - progress: 进度回调函数
	// 返回：
	//   - string: 下载文件的本地路径
	//   - error: 下载失败时返回错误
	Download(ctx context.Context, info *domain.UpdateInfo, progress DownloadProgress) (string, error)

	// Verify 验证下载文件的完整性（SHA256）。
	// 参数：
	//   - filePath: 下载文件路径
	//   - expectedSHA256: 期望的 SHA256 哈希值
	// 返回：
	//   - error: 验证失败时返回错误
	Verify(filePath string, expectedSHA256 string) error

	// Install 安装更新（原子替换当前二进制）。
	// 参数：
	//   - ctx: 上下文
	//   - filePath: 下载文件路径
	// 返回：
	//   - error: 安装失败时返回错误
	Install(ctx context.Context, filePath string) error

	// Rollback 回滚到上一版本。
	// 返回：
	//   - error: 回滚失败时返回错误
	Rollback() error

	// GetBackupDir 获取备份目录路径。
	GetBackupDir() string
}
