package updater

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"assh/asshc/domain"
)

// BinaryInstaller 实现 Updater 接口的安装和回滚功能。
//
// 安装策略：
//   - 备份当前二进制到备份目录
//   - 验证下载文件的可执行权限
//   - 原子替换：os.Rename 替换当前二进制
//   - 设置可执行权限 0755
//
// 回滚策略：
//   - 从备份目录查找最近备份
//   - 恢复备份文件到原位置
type BinaryInstaller struct {
	currentPath string // 当前二进制路径
	backupDir   string // 备份目录
	maxBackups  int    // 最大备份数量
}

// NewBinaryInstaller 创建新的二进制安装器。
// 参数：
//   - currentPath: 当前二进制文件路径
//   - backupDir: 备份目录路径
//   - maxBackups: 最大备份数量（默认 3）
func NewBinaryInstaller(currentPath, backupDir string, maxBackups int) *BinaryInstaller {
	if maxBackups <= 0 {
		maxBackups = 3
	}
	return &BinaryInstaller{
		currentPath: currentPath,
		backupDir:   backupDir,
		maxBackups:  maxBackups,
	}
}

// Install 安装更新（原子替换当前二进制）。
//
// 流程：
//  1. 备份当前二进制
//  2. 验证下载文件
//  3. 替换当前二进制
//  4. 清理旧备份
func (i *BinaryInstaller) Install(ctx context.Context, filePath string) error {
	// 1. 备份当前二进制
	backupPath, err := i.backup()
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// 2. 验证下载文件
	if err := i.verifyFile(filePath); err != nil {
		return err
	}

	// 3. 替换当前二进制
	if err := i.replace(filePath); err != nil {
		// 替换失败，尝试回滚
		if rollbackErr := i.restore(backupPath); rollbackErr != nil {
			return fmt.Errorf("replace failed and rollback failed: %v, rollback error: %v", err, rollbackErr)
		}
		return fmt.Errorf("replace failed, rolled back: %w", err)
	}

	// 4. 清理旧备份
	i.cleanupOldBackups()

	return nil
}

// Rollback 回滚到上一版本。
func (i *BinaryInstaller) Rollback() error {
	// 查找最近的备份
	backupPath, err := i.findLatestBackup()
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrRollbackFailed, err)
	}

	// 恢复备份
	if err := i.restore(backupPath); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrRollbackFailed, err)
	}

	return nil
}

// GetBackupDir 获取备份目录路径。
func (i *BinaryInstaller) GetBackupDir() string {
	return i.backupDir
}

// backup 备份当前二进制。
func (i *BinaryInstaller) backup() (string, error) {
	// 确保备份目录存在
	if err := os.MkdirAll(i.backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup dir: %w", err)
	}

	// 生成备份文件名
	version := i.getCurrentVersion()
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("assh-%s-%s.backup", version, timestamp)
	backupPath := filepath.Join(i.backupDir, backupName)

	// 复制当前二进制
	if err := copyFile(i.currentPath, backupPath); err != nil {
		return "", fmt.Errorf("failed to backup: %w", err)
	}

	// 设置权限
	if err := os.Chmod(backupPath, 0755); err != nil {
		return "", fmt.Errorf("failed to set permissions: %w", err)
	}

	return backupPath, nil
}

// verifyFile 验证下载文件。
func (i *BinaryInstaller) verifyFile(filePath string) error {
	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// 检查文件大小
	if info.Size() == 0 {
		return fmt.Errorf("%w: empty file", domain.ErrVerificationFailed)
	}

	// 在 Windows 上，无法直接检查可执行权限
	if runtime.GOOS != "windows" {
		// 检查文件是否可执行
		if info.Mode()&0111 == 0 {
			// 尝试添加可执行权限
			if err := os.Chmod(filePath, info.Mode()|0111); err != nil {
				return fmt.Errorf("failed to set executable permission: %w", err)
			}
		}
	}

	return nil
}

// replace 替换当前二进制。
func (i *BinaryInstaller) replace(filePath string) error {
	// 在 Windows 上，无法直接替换正在运行的二进制
	// 需要使用 rename + 删除策略
	if runtime.GOOS == "windows" {
		return i.replaceWindows(filePath)
	}

	// Unix 系统：原子替换
	if err := os.Rename(filePath, i.currentPath); err != nil {
		return fmt.Errorf("failed to replace: %w", err)
	}

	return nil
}

// replaceWindows 在 Windows 上替换二进制。
// Windows 无法直接替换正在运行的文件，需要使用 rename + 删除策略。
func (i *BinaryInstaller) replaceWindows(filePath string) error {
	// 1. 重命名当前二进制
	oldPath := i.currentPath + ".old"
	if err := os.Rename(i.currentPath, oldPath); err != nil {
		return fmt.Errorf("failed to rename current: %w", err)
	}

	// 2. 移动新二进制
	if err := os.Rename(filePath, i.currentPath); err != nil {
		// 回滚
		os.Rename(oldPath, i.currentPath)
		return fmt.Errorf("failed to move new: %w", err)
	}

	// 3. 删除旧二进制（可能失败，但不影响使用）
	os.Remove(oldPath)

	return nil
}

// restore 恢复备份文件。
func (i *BinaryInstaller) restore(backupPath string) error {
	if err := copyFile(backupPath, i.currentPath); err != nil {
		return fmt.Errorf("failed to restore: %w", err)
	}

	if err := os.Chmod(i.currentPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	return nil
}

// findLatestBackup 查找最近的备份文件。
func (i *BinaryInstaller) findLatestBackup() (string, error) {
	entries, err := os.ReadDir(i.backupDir)
	if err != nil {
		return "", fmt.Errorf("failed to read backup dir: %w", err)
	}

	var latestPath string
	var latestTime time.Time

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".backup" {
			continue
		}

		path := filepath.Join(i.backupDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if latestPath == "" || info.ModTime().After(latestTime) {
			latestPath = path
			latestTime = info.ModTime()
		}
	}

	if latestPath == "" {
		return "", fmt.Errorf("no backup found")
	}

	return latestPath, nil
}

// cleanupOldBackups 清理旧备份，保留最近的 maxBackups 个。
func (i *BinaryInstaller) cleanupOldBackups() {
	entries, err := os.ReadDir(i.backupDir)
	if err != nil {
		return
	}

	// 收集所有备份文件
	type backupFile struct {
		path    string
		modTime time.Time
	}

	var backups []backupFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".backup" {
			continue
		}

		path := filepath.Join(i.backupDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, backupFile{
			path:    path,
			modTime: info.ModTime(),
		})
	}

	// 按时间排序（最新的在前）
	for i := 0; i < len(backups)-1; i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[j].modTime.After(backups[i].modTime) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}

	// 删除多余的备份
	for i := i.maxBackups; i < len(backups); i++ {
		os.Remove(backups[i].path)
	}
}

// getCurrentVersion 从当前二进制路径提取版本号。
func (i *BinaryInstaller) getCurrentVersion() string {
	// 尝试从文件名提取版本
	// base := filepath.Base(i.currentPath)
	// 如果文件名包含版本号，提取它
	// 否则返回 "unknown"
	return "unknown"
}

// copyFile 复制文件。
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}
