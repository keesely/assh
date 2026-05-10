// Package service 实现业务用例编排层。
//
// 本服务层负责组合 port 接口来完成具体业务逻辑。
// 依赖的接口通过构造函数注入，不直接依赖具体的 infra 实现。
// 主要包含 ServerService（服务器配置管理）和 ConnectService（SSH 连接管理）。
package service

import (
	"assh/asshc/domain"
	"assh/asshc/port"
)

// ServerService 封装服务器配置管理的业务逻辑。
// 提供对 Server 实体的增删改查、分组管理、搜索和版本回滚功能。
type ServerService struct {
	repo port.ServerRepository
}

// NewServerService 创建 ServerService 实例，注入 ServerRepository 依赖。
func NewServerService(repo port.ServerRepository) *ServerService {
	return &ServerService{repo: repo}
}

// ListServers 返回所有服务器列表，按分组组织为二级映射。
func (s *ServerService) ListServers() (map[string]map[string]*domain.Server, error) {
	return s.repo.List()
}

// GetServer 根据完整名称获取单个服务器配置。
func (s *ServerService) GetServer(name string) (*domain.Server, error) {
	if name == "" {
		return nil, domain.ErrInvalidName
	}
	return s.repo.Get(name)
}

// AddServer 添加新服务器，如果已存在则返回 ErrExists。
// 自动设置默认值（port=22, user=root），并从名称中解析分组信息。
func (s *ServerService) AddServer(name string, server *domain.Server) error {
	if name == "" {
		return domain.ErrInvalidName
	}

	existing, err := s.repo.Get(name)
	if err == nil && existing != nil {
		return domain.ErrExists
	}

	if server == nil {
		server = &domain.Server{}
	}

	if server.Host == "" {
		return domain.ErrEmptyField
	}

	if server.Port == 0 {
		server.Port = 22
	}

	if server.User == "" {
		server.User = "root"
	}

	group, serverName := domain.ParseName(name)
	server.Group = group
	server.Name = serverName

	return s.repo.Set(name, server)
}

// UpdateServer 更新已有服务器的配置，服务器不存在时返回错误。
func (s *ServerService) UpdateServer(name string, server *domain.Server) error {
	if name == "" {
		return domain.ErrInvalidName
	}

	existing, err := s.repo.Get(name)
	if err != nil {
		return err
	}

	if existing == nil {
		return domain.ErrNotFound
	}

	if server == nil {
		server = &domain.Server{}
	}

	group, serverName := domain.ParseName(name)
	server.Group = group
	server.Name = serverName

	return s.repo.Set(name, server)
}

// SetServer 执行 upsert 操作：服务器不存在时创建，存在时更新。
// 创建时要求 host 必填，并设置默认值；更新时保留已有版本号，
// 调用方负责合并现有服务器的字段。
func (s *ServerService) SetServer(name string, server *domain.Server) error {
	if name == "" {
		return domain.ErrInvalidName
	}

	existing, err := s.repo.Get(name)
	isNew := err != nil

	if !isNew && existing != nil {
		// 更新模式：保留现有版本号
		server.Version = existing.Version
	} else {
		// 创建模式：校验必填字段
		if server.Host == "" {
			return domain.ErrEmptyField
		}
		if server.Port <= 0 || server.Port > 65535 {
			return domain.ErrInvalidPort
		}
		if server.User == "" {
			server.User = "root"
		}
		if server.Port == 0 {
			server.Port = 22
		}
	}

	group, serverName := domain.ParseName(name)
	server.Group = group
	server.Name = serverName

	return s.repo.Set(name, server)
}

// RemoveServer 删除指定名称的服务器配置，服务器不存在时返回错误。
func (s *ServerService) RemoveServer(name string) error {
	if name == "" {
		return domain.ErrInvalidName
	}

	_, err := s.repo.Get(name)
	if err != nil {
		return err
	}

	return s.repo.Delete(name)
}

// MoveServer 将服务器从 from 重命名/移动到 to（跨分组移动）。
func (s *ServerService) MoveServer(from, to string) error {
	if from == "" || to == "" {
		return domain.ErrInvalidName
	}

	_, err := s.repo.Get(from)
	if err != nil {
		return err
	}

	return s.repo.Move(from, to)
}

// SearchServers 按关键字搜索服务器。关键字为空时返回所有服务器。
func (s *ServerService) SearchServers(keyword string) (map[string]map[string]*domain.Server, error) {
	if keyword == "" {
		return s.repo.List()
	}
	return s.repo.Search(keyword)
}

// GetGroup 返回指定分组下的所有服务器。
func (s *ServerService) GetGroup(group string) (map[string]*domain.Server, error) {
	return s.repo.GetGroup(group)
}

// RollbackServer 将服务器配置回滚到指定的历史版本。
// version 必须大于等于 1，且该版本必须在变更日志中存在。
func (s *ServerService) RollbackServer(name string, version int) error {
	if name == "" {
		return domain.ErrInvalidName
	}
	if version < 1 {
		return domain.ErrInvalidVersion
	}
	_, err := s.repo.Get(name)
	if err != nil {
		return err
	}
	return s.repo.RollbackTo(name, version)
}

// GetServerChangelog 返回指定服务器的变更历史记录列表。
func (s *ServerService) GetServerChangelog(name string) ([]domain.ChangelogEntry, error) {
	if name == "" {
		return nil, domain.ErrInvalidName
	}
	return s.repo.GetChangelog(name)
}
