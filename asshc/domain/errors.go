package domain

import "errors"

// 领域层通用错误定义，用于在整个应用层传递明确的错误语义。
var (
	// ErrNotFound 表示请求的服务器不存在。
	ErrNotFound = errors.New("server not found")
	// ErrExists 表示尝试创建的服务器已存在。
	ErrExists = errors.New("server already exists")
	// ErrInvalidName 表示服务器名称为空或格式无效。
	ErrInvalidName = errors.New("invalid server name")
	// ErrInvalidPort 表示端口号超出有效范围（1-65535）。
	ErrInvalidPort = errors.New("invalid port number")
	// ErrEmptyField 表示必填字段为空。
	ErrEmptyField = errors.New("empty field not allowed")
	// ErrVersionNotFound 表示变更日志中未找到指定版本。
	ErrVersionNotFound = errors.New("version not found in changelog")
	// ErrInvalidVersion 表示版本号无效（小于 1）。
	ErrInvalidVersion = errors.New("invalid version number")
	// ErrKnownServerNotFound 表示 known_servers 表中未找到指定记录。
	ErrKnownServerNotFound = errors.New("known server not found")

	// ErrAccountNotFound 表示云账户未配置。
	ErrAccountNotFound = errors.New("cloud account not found")

	// ErrSyncFailed 表示同步操作失败。
	ErrSyncFailed = errors.New("sync operation failed")

	// ErrInvalidAccount 表示云账户凭据无效。
	ErrInvalidAccount = errors.New("invalid cloud account credentials")
)