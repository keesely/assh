package port

import "assh/asshc/domain"

// TunnelServiceRepository 定义隧道服务配置的持久化存储接口。
// 实现对 TunnelService 实体的增删改查、使用记录等功能。
// 具体实现由 infra/store 包提供（基于 SQLite）。
type TunnelServiceRepository interface {
	// Create 创建新的隧道服务配置。
	Create(svc *domain.TunnelService) error

	// Update 更新隧道服务配置。
	Update(svc *domain.TunnelService) error

	// Delete 删除指定 ID 的隧道服务配置。
	Delete(id int64) error

	// GetByID 根据 ID 获取隧道服务配置。
	GetByID(id int64) (*domain.TunnelService, error)

	// GetByName 根据分组名和服务名获取隧道服务配置。
	GetByName(groupName, name string) (*domain.TunnelService, error)

	// List 返回指定分组下的所有隧道服务配置。
	// groupName 为空时返回所有配置。
	List(groupName string) ([]*domain.TunnelService, error)

	// RecordUsage 记录一次使用，递增 use_count 并更新 last_used。
	RecordUsage(id int64) error
}
