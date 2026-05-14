// Package port 定义核心业务接口（端口），实现依赖倒置。
//
// 本包只定义接口和领域类型引用，不包含任何实现。
// service 层依赖本包的接口，infra 层提供具体实现，
// 从而实现依赖倒置原则——高层模块不依赖低层模块的具体实现。
package port

import "assh/asshc/domain"

// ServerRepository 定义服务器配置的持久化存储接口。
// 实现对 Server 实体的增删改查、分组管理、搜索、版本回滚等功能。
// 具体实现由 infra/store 包提供（基于 SQLite）。
type ServerRepository interface {
	// List 返回所有服务器，按分组名 -> 服务器名 -> Server 的二级映射组织。
	List() (map[string]map[string]*domain.Server, error)
	// Get 根据完整名称（group.name）获取单个服务器配置。
	Get(name string) (*domain.Server, error)
	// Set 保存或更新服务器配置（upsert 语义）。
	Set(name string, server *domain.Server) error
	// Delete 删除指定名称的服务器配置。
	Delete(name string) error
	// Move 将服务器从 from 重命名/移动到 to。
	Move(from, to string) error
	// Search 按关键字搜索服务器名称、主机地址和备注。
	Search(keyword string) (map[string]map[string]*domain.Server, error)
	// GetGroup 返回指定分组下的所有服务器。
	GetGroup(group string) (map[string]*domain.Server, error)
	// GetChangelog 返回指定服务器的变更历史记录。
	GetChangelog(name string) ([]domain.ChangelogEntry, error)
	// RollbackTo 将服务器配置回滚到指定版本。
	RollbackTo(name string, version int) error
	// Close 关闭存储连接，释放资源。
	Close() error
}

// KnownServerRecorder 定义未具名服务器直连记录的存储接口。
// 用于自动记录直连（user@host / -H host）的连接历史和密钥关联。
// 实现 ADR-018 的 known-servers 隐性表设计。
type KnownServerRecorder interface {
	// RecordDirectConnect 记录或更新一次直连操作。
	// 如果同一 ID 已存在，递增 connect_count 并更新 last_connected_at；
	// 如果不存在，创建新记录。
	RecordDirectConnect(ks *domain.KnownServer) error
	// LookupKnownServer 根据 ID 查找已知服务器记录。
	LookupKnownServer(id string) (*domain.KnownServer, error)
	// LookupKnownServerByAuth 根据主机+端口+用户+认证指纹查找记录。
	// 内部计算 ID 后调用 LookupKnownServer。
	LookupKnownServerByAuth(user, host string, port int, authFingerprint string) (*domain.KnownServer, error)
	// UpdateKeyBackupPath 更新已知服务器的密钥备份路径。
	UpdateKeyBackupPath(id, keyBackupPath string) error
	// DeleteKnownServer 删除指定 ID 的已知服务器记录。
	DeleteKnownServer(id string) error
	// ListKnownServers 返回所有已知服务器记录。
	ListKnownServers() ([]*domain.KnownServer, error)
}