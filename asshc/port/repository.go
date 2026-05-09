package port

import "assh/asshc/domain"

type ServerRepository interface {
	List() (map[string]map[string]*domain.Server, error)
	Get(name string) (*domain.Server, error)
	Set(name string, server *domain.Server) error
	Delete(name string) error
	Move(from, to string) error
	Search(keyword string) (map[string]map[string]*domain.Server, error)
	GetGroup(group string) (map[string]*domain.Server, error)
	GetChangelog(name string) ([]domain.ChangelogEntry, error)
	RollbackTo(name string, version int) error
	Close() error
}