package domain

import "time"

// UpdateInfo 表示更新信息。
type UpdateInfo struct {
	Version      string    `json:"version"`       // 新版本号
	ReleaseDate  time.Time `json:"release_date"`  // 发布时间
	ReleaseNotes string    `json:"release_notes"` // 发布说明
	DownloadURL  string    `json:"download_url"`  // 下载链接
	SHA256       string    `json:"sha256"`        // 文件哈希值
	FileSize     int64     `json:"file_size"`     // 文件大小（字节）
	Mandatory    bool      `json:"mandatory"`     // 是否强制更新
}

// UpdatePlatform 表示目标平台信息。
type UpdatePlatform struct {
	OS   string // 操作系统 (darwin, linux, windows)
	Arch string // 架构 (amd64, arm64)
}
