package service

import (
	"assh/asshc/domain"
	"assh/asshc/port"
)

type ServerService struct {
	repo port.ServerRepository
}

func NewServerService(repo port.ServerRepository) *ServerService {
	return &ServerService{repo: repo}
}

func (s *ServerService) ListServers() (map[string]map[string]*domain.Server, error) {
	return s.repo.List()
}

func (s *ServerService) GetServer(name string) (*domain.Server, error) {
	if name == "" {
		return nil, domain.ErrInvalidName
	}
	return s.repo.Get(name)
}

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

// SetServer performs an upsert: creates if not exists, updates if exists.
// For creation, host is required; for update, only specified fields are changed
// (caller is responsible for merging from existing server).
// Validation ensures basic field integrity.
func (s *ServerService) SetServer(name string, server *domain.Server) error {
	if name == "" {
		return domain.ErrInvalidName
	}

	existing, err := s.repo.Get(name)
	isNew := err != nil

	if !isNew && existing != nil {
		// Update: preserve version from existing
		server.Version = existing.Version
	} else {
		// Create: require host
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

func (s *ServerService) SearchServers(keyword string) (map[string]map[string]*domain.Server, error) {
	if keyword == "" {
		return s.repo.List()
	}
	return s.repo.Search(keyword)
}

func (s *ServerService) GetGroup(group string) (map[string]*domain.Server, error) {
	return s.repo.GetGroup(group)
}

// RollbackServer rolls back a server to the specified version.
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

// GetServerChangelog returns the change history for a server.
func (s *ServerService) GetServerChangelog(name string) ([]domain.ChangelogEntry, error) {
	if name == "" {
		return nil, domain.ErrInvalidName
	}
	return s.repo.GetChangelog(name)
}
