// Package port 定义了应用程序的端口（接口）层。
//
// 该层包含所有外部依赖的接口定义，遵循依赖倒置原则。
// 具体实现位于 infra 层，service 层只依赖 port 接口。
package port

import (
	"context"
	"time"
)

// Syncer 定义了云同步的抽象接口。
//
// 该接口抽象了与云存储服务的交互，支持数据的上传、下载和管理。
// 具体实现可以是七牛云、阿里云OSS、AWS S3等。
type Syncer interface {
	// Login 验证云账户凭据并建立连接。
	//
	// 参数：
	//   ctx - 上下文，用于控制超时和取消
	//   account - 云账户信息，包含访问密钥、秘密密钥、存储空间等
	//
	// 返回：
	//   error - 验证失败时返回错误，成功时返回 nil
	Login(ctx context.Context, account SyncAccount) error

	// Push 将本地数据推送到云端。
	//
	// 该方法执行增量同步，只推送变更的数据。
	// 首次同步或数据冲突时可能执行全量同步。
	//
	// 参数：
	//   ctx - 上下文，用于控制超时和取消
	//   data - 要推送的数据内容
	//
	// 返回：
	//   *SyncResult - 同步操作的详细结果
	//   error - 操作失败时返回错误
	Push(ctx context.Context, data *SyncData) (*SyncResult, error)

	// Pull 从云端拉取数据到本地。
	//
	// 该方法执行增量同步，只拉取云端新增或变更的数据。
	//
	// 参数：
	//   ctx - 上下文，用于控制超时和取消
	//
	// 返回：
	//   *SyncData - 从云端拉取的数据内容
	//   error - 操作失败时返回错误
	Pull(ctx context.Context) (*SyncData, error)

	// List 列出云端所有同步对象。
	//
	// 返回云端存储的所有数据对象，包括版本信息、大小、修改时间等。
	//
	// 参数：
	//   ctx - 上下文，用于控制超时和取消
	//
	// 返回：
	//   []*SyncObject - 云端对象列表
	//   error - 操作失败时返回错误
	List(ctx context.Context) ([]*SyncObject, error)

	// Delete 删除云端的同步对象。
	//
	// 根据指定的键名删除云端的数据对象。
	//
	// 参数：
	//   ctx - 上下文，用于控制超时和取消
	//   key - 要删除的对象键名
	//
	// 返回：
	//   error - 操作失败时返回错误
	Delete(ctx context.Context, key string) error
}

// SyncAccount 表示云账户信息。
//
// 包含连接云存储服务所需的所有凭据和配置信息。
type SyncAccount struct {
	// Type 云服务类型，如 "qiniu"
	Type string `json:"type"`

	// AccessKey 访问密钥
	AccessKey string `json:"access_key"`

	// SecretKey 秘密密钥（加密存储）
	SecretKey string `json:"secret_key"`

	// Bucket 存储空间名称
	Bucket string `json:"bucket"`

	// Zone 区域：huadong/huabei/huanan/beimei/xinjiapo
	Zone string `json:"zone"`
}

// SyncData 表示同步的数据内容。
//
// 包含所有需要同步的服务器配置数据，以及元数据信息。
type SyncData struct {
	// Version 数据格式版本，用于兼容性处理
	Version string `json:"version"`

	// Timestamp 同步时间戳，用于增量同步判断
	Timestamp time.Time `json:"timestamp"`

	// Servers 服务器配置数据，键为服务器名称
	Servers map[string]*ServerData `json:"servers"`

	// Checksum 数据校验和，用于验证数据完整性
	Checksum string `json:"checksum"`
}

// ServerData 表示单个服务器的同步数据。
//
// 包含服务器的所有配置信息，用于云端存储和同步。
type ServerData struct {
	// Name 服务器名称
	Name string `json:"name"`

	// Group 服务器分组
	Group string `json:"group"`

	// Host 服务器地址
	Host string `json:"host"`

	// Port SSH端口
	Port int `json:"port"`

	// User 登录用户名
	User string `json:"user"`

	// Password 加密后的密码
	Password string `json:"password,omitempty"`

	// KeyFile SSH私钥文件路径
	KeyFile string `json:"key_file,omitempty"`

	// Remark 服务器备注
	Remark string `json:"remark,omitempty"`

	// Options 额外选项
	Options map[string]interface{} `json:"options,omitempty"`

	// Version 配置版本号
	Version int `json:"version"`

	// UpdatedAt 最后更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// SyncResult 表示同步操作的结果。
//
// 包含同步操作的详细统计信息和结果状态。
type SyncResult struct {
	// Success 同步是否成功
	Success bool `json:"success"`

	// Message 结果消息
	Message string `json:"message"`

	// Pushed 推送的服务器数量
	Pushed int `json:"pushed"`

	// Updated 更新的服务器数量
	Updated int `json:"updated"`

	// Conflicts 冲突数量
	Conflicts int `json:"conflicts"`

	// Timestamp 同步完成时间
	Timestamp time.Time `json:"timestamp"`
}

// SyncObject 表示云端的同步对象。
//
// 包含云端存储对象的基本信息。
type SyncObject struct {
	// Key 对象键名
	Key string `json:"key"`

	// Size 对象大小（字节）
	Size int64 `json:"size"`

	// ETag 对象的ETag标识
	ETag string `json:"etag"`

	// Modified 最后修改时间
	Modified time.Time `json:"modified"`

	// Version 数据格式版本
	Version string `json:"version"`
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
//
// 记录每次同步操作的详细信息。
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