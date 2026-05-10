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