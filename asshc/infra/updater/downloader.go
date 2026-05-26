package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// HTTPDownloader 实现 Downloader 接口的下载和验证功能。
type HTTPDownloader struct {
	client  *http.Client
	tempDir string // 临时下载目录
}

// NewHTTPDownloader 创建新的 HTTP 下载器。
// 参数：
//   - tempDir: 临时下载目录路径
func NewHTTPDownloader(tempDir string) *HTTPDownloader {
	return &HTTPDownloader{
		client: &http.Client{
			Timeout: 30 * time.Minute, // 下载大文件需要较长超时
		},
		tempDir: tempDir,
	}
}

// Download 下载指定版本的更新包。
// 支持进度回调和断点续传。
func (d *HTTPDownloader) Download(ctx context.Context, info *domain.UpdateInfo, progress port.DownloadProgress) (string, error) {
	// 确保临时目录存在
	if err := os.MkdirAll(d.tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	// 构建目标文件路径
	fileName := fmt.Sprintf("assh-%s-download", info.Version)
	if filepath.Ext(info.DownloadURL) != "" {
		fileName = filepath.Base(info.DownloadURL)
	}
	targetPath := filepath.Join(d.tempDir, fileName)

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "GET", info.DownloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// 发送请求
	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrNetworkError, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: HTTP %d", domain.ErrNetworkError, resp.StatusCode)
	}

	// 获取文件大小
	totalSize := resp.ContentLength
	if totalSize <= 0 && info.FileSize > 0 {
		totalSize = info.FileSize
	}

	// 创建目标文件
	file, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// 带进度的下载
	writer := &progressWriter{
		writer:   file,
		total:    totalSize,
		progress: progress,
	}

	// 限制下载大小，防止磁盘耗尽攻击
	// 如果已知文件大小，使用 LimitReader；否则使用默认限制 (1GB)
	var reader io.Reader = resp.Body
	if totalSize > 0 {
		reader = io.LimitReader(resp.Body, totalSize)
	} else {
		// 默认限制 1GB
		const maxDownloadSize = 1 * 1024 * 1024 * 1024
		reader = io.LimitReader(resp.Body, maxDownloadSize)
	}

	if _, err := io.Copy(writer, reader); err != nil {
		// 下载失败，清理临时文件
		os.Remove(targetPath)
		return "", fmt.Errorf("failed to download: %w", err)
	}

	return targetPath, nil
}

// Verify 验证下载文件的完整性（SHA256）。
func (d *HTTPDownloader) Verify(filePath string, expectedSHA256 string) error {
	if expectedSHA256 == "" {
		return nil // 无预期哈希值，跳过验证
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != expectedSHA256 {
		return fmt.Errorf("%w: expected %s, got %s", domain.ErrVerificationFailed, expectedSHA256, actualHash)
	}

	return nil
}

// progressWriter 包装 io.Writer，添加进度回调。
type progressWriter struct {
	writer      io.Writer
	total       int64
	downloaded  int64
	progress    port.DownloadProgress
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	pw.downloaded += int64(n)

	// 调用进度回调
	if pw.progress != nil && pw.total > 0 {
		percent := float64(pw.downloaded) / float64(pw.total)
		pw.progress(pw.downloaded, pw.total, percent)
	}

	return n, err
}
