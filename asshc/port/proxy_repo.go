// Package port 定义核心业务接口（端口），实现依赖倒置。
//
// 本包只定义接口和领域类型引用，不包含任何实现。
// service 层依赖本包的接口，infra 层提供具体实现，
// 从而实现依赖倒置原则——高层模块不依赖低层模块的具体实现。
package port

import "assh/asshc/domain"

// ProxyServiceRepository 定义代理服务配置的持久化存储接口。
// 实现对 ProxyService 实体的增删改查、使用记录等功能。
// 具体实现由 infra/store 包提供（基于 SQLite）。
type ProxyServiceRepository interface {
	// Create 创建新的代理服务配置。
	Create(svc *domain.ProxyService) error

	// Update 更新代理服务配置。
	Update(svc *domain.ProxyService) error

	// Delete 删除指定 ID 的代理服务配置。
	Delete(id int64) error

	// GetByID 根据 ID 获取代理服务配置。
	GetByID(id int64) (*domain.ProxyService, error)

	// GetByName 根据分组名和服务名获取代理服务配置。
	GetByName(groupName, name string) (*domain.ProxyService, error)

	// List 返回指定分组下的所有代理服务配置。
	// groupName 为空时返回所有配置。
	List(groupName string) ([]*domain.ProxyService, error)

	// RecordUsage 记录一次使用，递增 use_count 并更新 last_used。
	RecordUsage(id int64) error
}