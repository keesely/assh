// Package service 实现业务用例编排层。
//
// 本服务层负责组合 port 接口来完成具体业务逻辑。
// 依赖的接口通过构造函数注入，不直接依赖具体的 infra 实现。
// 主要包含 ServerService（服务器配置管理）和 ConnectService（SSH 连接管理）。
package service

import (
	"math"
	"sort"
	"strings"

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

// SuggestServer 在精确查找失败时，返回最接近的服务器名作为建议。
// 使用 Levenshtein 编辑距离进行模糊匹配，同时匹配全名（group.name）和裸名（name）。
// 距离阈值取 len(input) 的 60%（至少 2），在此范围内返回最佳匹配。
// 返回建议的全名（含分组前缀）和建议是否有效。
func (s *ServerService) SuggestServer(input string) (string, bool) {
	if input == "" {
		return "", false
	}
	all, err := s.repo.List()
	if err != nil || len(all) == 0 {
		return "", false
	}

	// Step 1: prefix match — collect all prefix matches, pick shortest (closest match)
	// Sort for determinism. Shortest name is most likely what the user intended.
	inputLower := strings.ToLower(input)
	var prefixMatches []string
	for group, servers := range all {
		for name := range servers {
			fullName := domain.JoinName(group, name)
			if strings.HasPrefix(strings.ToLower(name), inputLower) ||
				strings.HasPrefix(strings.ToLower(fullName), inputLower) {
				prefixMatches = append(prefixMatches, fullName)
			}
		}
	}
	if len(prefixMatches) > 0 {
		sort.Slice(prefixMatches, func(i, j int) bool {
			if len(prefixMatches[i]) != len(prefixMatches[j]) {
				return len(prefixMatches[i]) < len(prefixMatches[j])
			}
			return prefixMatches[i] < prefixMatches[j]
		})
		return prefixMatches[0], true
	}

	// Step 2: Levenshtein fuzzy match against full name and bare name
	threshold := math.Max(2, float64(len(input))*0.6)
	bestName := ""
	bestDist := math.MaxInt32

	for group, servers := range all {
		for name := range servers {
			fullName := domain.JoinName(group, name)
			dist := levenshtein(input, fullName)
			if d2 := levenshtein(input, name); d2 < dist {
				dist = d2
			}
			if dist < bestDist {
				bestDist = dist
				bestName = fullName
			}
		}
	}

	if bestDist <= int(threshold) {
		return bestName, true
	}
	return "", false
}

// levenshtein 计算两个字符串之间的编辑距离（Levenshtein distance）。
// 使用经典 DP 算法，时间复杂度 O(len(a)*len(b))。
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// 优化：用一维数组代替二维矩阵
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}
