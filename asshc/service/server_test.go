package service

import (
	"testing"

	"assh/asshc/domain"
)

type mockRepo struct {
	servers map[string]map[string]*domain.Server
}

func (m *mockRepo) List() (map[string]map[string]*domain.Server, error) {
	return m.servers, nil
}

func (m *mockRepo) Get(name string) (*domain.Server, error) {
	for _, group := range m.servers {
		if s, ok := group[name]; ok {
			return s, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockRepo) Set(name string, server *domain.Server) error {
	group, serverName := domain.ParseName(name)
	server.Name = serverName
	server.Group = group

	if m.servers == nil {
		m.servers = make(map[string]map[string]*domain.Server)
	}
	if m.servers[group] == nil {
		m.servers[group] = make(map[string]*domain.Server)
	}
	m.servers[group][serverName] = server
	return nil
}

func (m *mockRepo) Delete(name string) error {
	group, serverName := domain.ParseName(name)
	if m.servers[group] != nil {
		if _, ok := m.servers[group][serverName]; ok {
			delete(m.servers[group], serverName)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (m *mockRepo) Move(from, to string) error {
	s, err := m.Get(from)
	if err != nil {
		return err
	}
	m.Set(to, s)
	return m.Delete(from)
}

func (m *mockRepo) Search(keyword string) (map[string]map[string]*domain.Server, error) {
	result := make(map[string]map[string]*domain.Server)
	for g, servers := range m.servers {
		for n, s := range servers {
			if contains(keyword, s.Name) || contains(keyword, s.Host) || contains(keyword, s.Remark) {
				if result[g] == nil {
					result[g] = make(map[string]*domain.Server)
				}
				result[g][n] = s
			}
		}
	}
	return result, nil
}

func (m *mockRepo) GetGroup(group string) (map[string]*domain.Server, error) {
	return m.servers[group], nil
}

func (m *mockRepo) GetChangelog(name string) ([]domain.ChangelogEntry, error) {
	return nil, domain.ErrNotFound
}

func (m *mockRepo) RollbackTo(name string, version int) error {
	return domain.ErrNotFound
}

func (m *mockRepo) Close() error { return nil }

func contains(substr, str string) bool {
	return len(str) > 0 && len(substr) > 0 && 
	       (len(str) >= len(substr) && 
	        (str[:len(substr)] == substr || 
		         (len(str) > len(substr) && (contains(substr, str[1:]) || contains(substr, str[:len(str)-1])))))
}

func TestNewServerService(t *testing.T) {
	repo := &mockRepo{}
	svc := NewServerService(repo)
	if svc == nil {
		t.Error("NewServerService should not return nil")
	}
}

func TestAddServerSuccess(t *testing.T) {
	repo := &mockRepo{}
	svc := NewServerService(repo)

	server := &domain.Server{
		Name: "web-01",
		Host: "192.168.1.1",
		Port: 22,
		User: "root",
	}

	err := svc.AddServer("web-01", server)
	if err != nil {
		t.Errorf("AddServer failed: %v", err)
	}
}

func TestAddServerDuplicate(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"web-01": {Name: "web-01", Host: "192.168.1.1"}},
		},
	}
	svc := NewServerService(repo)

	server := &domain.Server{
		Name: "web-01",
		Host: "192.168.1.2",
	}

	err := svc.AddServer("web-01", server)
	if err != domain.ErrExists {
		t.Errorf("expected ErrExists, got %v", err)
	}
}

func TestGetServer(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"web-01": {Name: "web-01", Host: "192.168.1.1"}},
		},
	}
	svc := NewServerService(repo)

	server, err := svc.GetServer("web-01")
	if err != nil {
		t.Errorf("GetServer failed: %v", err)
	}
	if server.Host != "192.168.1.1" {
		t.Errorf("host = %q, want %q", server.Host, "192.168.1.1")
	}
}

func TestGetServerNotFound(t *testing.T) {
	repo := &mockRepo{}
	svc := NewServerService(repo)

	_, err := svc.GetServer("nonexistent")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRemoveServer(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"web-01": {Name: "web-01", Host: "192.168.1.1"}},
		},
	}
	svc := NewServerService(repo)

	err := svc.RemoveServer("web-01")
	if err != nil {
		t.Errorf("RemoveServer failed: %v", err)
	}

	_, err = svc.GetServer("web-01")
	if err != domain.ErrNotFound {
		t.Error("server should be deleted")
	}
}

func TestMoveServer(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"old-server": {Name: "old-server", Host: "192.168.1.1"}},
		},
	}
	svc := NewServerService(repo)

	err := svc.MoveServer("old-server", "new-server")
	if err != nil {
		t.Errorf("MoveServer failed: %v", err)
	}

	_, err = svc.GetServer("old-server")
	if err != domain.ErrNotFound {
		t.Error("old server should not exist")
	}

	_, err = svc.GetServer("new-server")
	if err != nil {
		t.Errorf("new server should exist: %v", err)
	}
}

func TestListServers(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"prod": {"web-01": {Name: "web-01", Host: "192.168.1.1"}},
			"dev":  {"api-01": {Name: "api-01", Host: "10.0.0.1"}},
		},
	}
	svc := NewServerService(repo)

	servers, err := svc.ListServers()
	if err != nil {
		t.Errorf("ListServers failed: %v", err)
	}

	if len(servers) != 2 {
		t.Errorf("expected 2 groups, got %d", len(servers))
	}
}

func TestSearchServers(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {
				"web-01": {Name: "web-01", Host: "192.168.1.1"},
				"db-01":  {Name: "db-01", Host: "192.168.1.2"},
			},
		},
	}
	svc := NewServerService(repo)

	results, err := svc.SearchServers("web")
	if err != nil {
		t.Errorf("SearchServers failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("should find at least one result")
	}
}