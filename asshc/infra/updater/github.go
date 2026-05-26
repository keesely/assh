package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"assh/asshc/domain"
)

// GitHubRelease 表示 GitHub Release API 响应。
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// GitHubChecker 实现 VersionChecker 接口，从 GitHub 检查更新。
//
// 版本检查策略：
//   - 通过获取 GitHub 仓库中的特定文件（如 version.txt）检查最新版本
//   - 文件格式：第一行为版本号（如 v2.0.2）
//
// 下载策略：
//   - 通过 GitHub Release API 查找适配当前系统和架构的资产
type GitHubChecker struct {
	owner       string // 仓库所有者
	repo        string // 仓库名称
	current     string // 当前版本
	versionFile string // 版本文件路径（如 "version.txt"）
	client      *http.Client
}

// NewGitHubChecker 创建新的 GitHub 版本检查器。
// 参数：
//   - owner: 仓库所有者
//   - repo: 仓库名称
//   - current: 当前版本号
//   - versionFile: 版本文件路径（如 "version.txt"）
func NewGitHubChecker(owner, repo, current, versionFile string) *GitHubChecker {
	return &GitHubChecker{
		owner:       owner,
		repo:        repo,
		current:     current,
		versionFile: versionFile,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CheckUpdate 从 GitHub 仓库中的版本文件检查是否有新版本可用。
//
// 版本文件格式：
//
//	v2.0.2
//	# 其他注释内容（可选）
//
// 比较逻辑：
//   - 忽略 v 前缀
//   - 语义化版本比较（Major.Minor.Patch）
func (c *GitHubChecker) CheckUpdate() (*domain.UpdateInfo, error) {
	// 1. 从版本文件获取最新版本号
	latestVersion, err := c.fetchLatestVersion()
	if err != nil {
		return nil, err
	}

	// 2. 比较版本
	currentVersion := strings.TrimPrefix(c.current, "v")
	if !isNewerVersion(currentVersion, latestVersion) {
		return nil, domain.ErrAlreadyLatest
	}

	// 3. 获取 Release 信息（下载链接、文件大小等）
	return c.fetchReleaseInfo(latestVersion)
}

// fetchLatestVersion 从 GitHub 仓库中的版本文件获取最新版本号。
func (c *GitHubChecker) fetchLatestVersion() (string, error) {
	// 构建版本文件 URL（使用 raw.githubusercontent.com）
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/%s",
		c.owner, c.repo, c.versionFile)

	resp, err := c.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrNetworkError, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("%w: version file not found", domain.ErrUpdateNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: HTTP %d", domain.ErrNetworkError, resp.StatusCode)
	}

	// 读取文件内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// 解析第一行作为版本号
	lines := strings.SplitN(string(body), "\n", 2)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return "", fmt.Errorf("%w: empty version file", domain.ErrUpdateNotFound)
	}

	version := strings.TrimSpace(lines[0])
	version = strings.TrimPrefix(version, "v")

	return version, nil
}

// fetchReleaseInfo 从 GitHub Release API 获取指定版本的下载信息。
func (c *GitHubChecker) fetchReleaseInfo(version string) (*domain.UpdateInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/v%s",
		c.owner, c.repo, version)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrNetworkError, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: release v%s not found", domain.ErrUpdateNotFound, version)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", domain.ErrNetworkError, resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 查找当前平台的资产
	platform := c.GetPlatform()
	asset := c.findAsset(release.Assets, platform.OS, platform.Arch)
	if asset == nil {
		return nil, fmt.Errorf("%w: no asset found for %s/%s", domain.ErrUpdateNotFound, platform.OS, platform.Arch)
	}

	return &domain.UpdateInfo{
		Version:      version,
		ReleaseDate:  release.PublishedAt,
		ReleaseNotes: release.Body,
		DownloadURL:  asset.BrowserDownloadURL,
		SHA256:       "", // 从 release notes 或单独文件获取
		FileSize:     asset.Size,
		Mandatory:    false,
	}, nil
}

// findAsset 查找适配指定平台的资产。
func (c *GitHubChecker) findAsset(assets []struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}, os, arch string) *struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
} {
	// 资产命名格式：assh-{os}-{arch}[.exe]
	patterns := []string{
		fmt.Sprintf("assh-%s-%s", os, arch),
		fmt.Sprintf("%s-%s", os, arch),
	}

	for i, asset := range assets {
		name := strings.ToLower(asset.Name)
		for _, pattern := range patterns {
			if strings.Contains(name, pattern) {
				return &assets[i]
			}
		}
	}

	return nil
}

// GetCurrentVersion 获取当前版本号。
func (c *GitHubChecker) GetCurrentVersion() string {
	return c.current
}

// GetPlatform 获取当前平台信息。
func (c *GitHubChecker) GetPlatform() domain.UpdatePlatform {
	return domain.UpdatePlatform{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
}

// isNewerVersion 比较两个版本号，判断 latest 是否比 current 更新。
// 版本格式：Major.Minor.Patch
func isNewerVersion(current, latest string) bool {
	currentParts := parseVersion(current)
	latestParts := parseVersion(latest)

	// 比较 Major
	if latestParts[0] > currentParts[0] {
		return true
	}
	if latestParts[0] < currentParts[0] {
		return false
	}

	// 比较 Minor
	if latestParts[1] > currentParts[1] {
		return true
	}
	if latestParts[1] < currentParts[1] {
		return false
	}

	// 比较 Patch
	return latestParts[2] > currentParts[2]
}

// parseVersion 解析版本号字符串为 [Major, Minor, Patch]。
func parseVersion(version string) [3]int {
	var result [3]int
	parts := strings.SplitN(version, ".", 3)

	for i, part := range parts {
		if i >= 3 {
			break
		}
		// 忽略解析错误，默认为 0
		fmt.Sscanf(part, "%d", &result[i])
	}

	return result
}
