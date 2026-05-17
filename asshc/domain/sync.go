package domain

import "time"

// CloudAccount 表示云账户信息。
//
// 用于连接云存储服务（如七牛云）进行数据备份和同步。
// SecretKey 在持久化时使用 AES-GCM 加密存储。
type CloudAccount struct {
	// ID 数据库主键
	ID int `json:"id"`

	// Name 账户名称，默认 "default"
	Name string `json:"name"`

	// Type 云服务类型，如 "qiniu"
	Type string `json:"type"`

	// AccessKey 访问密钥
	AccessKey string `json:"access_key"`

	// SecretKey 秘密密钥（加密存储）
	SecretKey string `json:"-"`

	// Bucket 存储空间名称
	Bucket string `json:"bucket"`

	// Zone 区域：huadong/huabei/huanan/beimei/xinjiapo
	Zone string `json:"zone"`

	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// SyncDirection 同步方向枚举。
type SyncDirection string

const (
	// SyncDirectionPush 推送到云端
	SyncDirectionPush SyncDirection = "push"

	// SyncDirectionPull 从云端拉取
	SyncDirectionPull SyncDirection = "pull"
)

// SyncStatus 同步状态枚举。
type SyncStatus string

const (
	// SyncStatusSuccess 同步成功
	SyncStatusSuccess SyncStatus = "success"

	// SyncStatusFailed 同步失败
	SyncStatusFailed SyncStatus = "failed"

	// SyncStatusPartial 部分成功
	SyncStatusPartial SyncStatus = "partial"
)

// SyncHistory 表示同步历史记录。
type SyncHistory struct {
	// ID 记录ID
	ID int `json:"id"`

	// Direction 同步方向
	Direction SyncDirection `json:"direction"`

	// Status 同步状态
	Status SyncStatus `json:"status"`

	// Message 结果消息
	Message string `json:"message"`

	// Pushed 推送数量
	Pushed int `json:"pushed"`

	// Updated 更新数量
	Updated int `json:"updated"`

	// Conflicts 冲突数量
	Conflicts int `json:"conflicts"`

	// Timestamp 同步时间
	Timestamp time.Time `json:"timestamp"`
}
