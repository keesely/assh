package service

import (
	"fmt"
	"sync"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// HealthService 封装服务器健康检查的业务逻辑。
// 支持单一检查、分组检查和全量检查，支持并发。
type HealthService struct {
	checker port.HealthChecker
	repo    port.ServerRepository
}

// NewHealthService 创建 HealthService 实例。
func NewHealthService(checker port.HealthChecker, repo port.ServerRepository) *HealthService {
	return &HealthService{
		checker: checker,
		repo:    repo,
	}
}

// CheckServer 对指定名称的服务器执行健康检查。
// detail 为 true 时采集远程系统信息。
func (s *HealthService) CheckServer(name string, detail bool) (*port.HealthResult, error) {
	server, err := s.repo.Get(name)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, domain.ErrNotFound
	}
	return s.checker.Check(server, detail)
}

// CheckGroup 对指定分组下的所有服务器执行并发健康检查。
// concurrency 控制最大并发数（默认 5）。
func (s *HealthService) CheckGroup(group string, detail bool, concurrency int) ([]*port.HealthResult, error) {
	servers, err := s.repo.GetGroup(group)
	if err != nil {
		return nil, fmt.Errorf("failed to get group %q: %w", group, err)
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers in group %q", group)
	}

	serverList := make([]*domain.Server, 0, len(servers))
	for _, s := range servers {
		serverList = append(serverList, s)
	}

	return s.checkBatch(serverList, detail, concurrency)
}

// CheckAll 对所有服务器执行并发健康检查。
// concurrency 控制最大并发数（默认 5）。
func (s *HealthService) CheckAll(detail bool, concurrency int) ([]*port.HealthResult, error) {
	groups, err := s.repo.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	var serverList []*domain.Server
	for _, groupServers := range groups {
		for _, server := range groupServers {
			serverList = append(serverList, server)
		}
	}

	if len(serverList) == 0 {
		return nil, fmt.Errorf("no servers configured")
	}

	return s.checkBatch(serverList, detail, concurrency)
}

// CheckServers 对指定名称列表的服务器执行并发健康检查。
func (s *HealthService) CheckServers(names []string, detail bool, concurrency int) ([]*port.HealthResult, error) {
	var serverList []*domain.Server
	for _, name := range names {
		server, err := s.repo.Get(name)
		if err != nil {
			return nil, fmt.Errorf("server %q: %w", name, err)
		}
		if server == nil {
			return nil, fmt.Errorf("server %q not found", name)
		}
		serverList = append(serverList, server)
	}

	return s.checkBatch(serverList, detail, concurrency)
}

// checkBatch 并发执行批量健康检查。
func (s *HealthService) checkBatch(servers []*domain.Server, detail bool, concurrency int) ([]*port.HealthResult, error) {
	if concurrency <= 0 {
		concurrency = 5
	}

	results := make([]*port.HealthResult, len(servers))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, server := range servers {
		wg.Add(1)
		go func(idx int, srv *domain.Server) {
			defer wg.Done()
			sem <- struct{}{}        // 获取信号量
			defer func() { <-sem }() // 释放信号量

			result, err := s.checker.Check(srv, detail)
			if err != nil {
				// 构造错误结果
				result = &port.HealthResult{
					Server: domain.JoinName(srv.Group, srv.Name),
					Host:   srv.Host,
					Port:   srv.Port,
					Status: port.HealthStatusError,
					Error:  err.Error(),
				}
			}
			results[idx] = result
		}(i, server)
	}

	wg.Wait()
	return results, nil
}
