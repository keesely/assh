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
		return nil, domain.ErrNotFound
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