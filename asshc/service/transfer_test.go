package service

import (
	"testing"

	"assh/asshc/domain"
)

type mockTransferRepo struct {
	servers map[string]map[string]*domain.Server
}

func newMockTransferRepo() *mockTransferRepo {
	return &mockTransferRepo{
		servers: map[string]map[string]*domain.Server{
			"test": {
				"server1": {
					Name: "test.server1",
					Host: "192.168.1.100",
					Port: 22,
					User: "root",
				},
			},
		},
	}
}

func (r *mockTransferRepo) List() (map[string]map[string]*domain.Server, error) {
	return r.servers, nil
}

func (r *mockTransferRepo) Get(name string) (*domain.Server, error) {
	group, srv := domain.ParseName(name)
	if groupServers, ok := r.servers[group]; ok {
		if server, ok := groupServers[srv]; ok {
			return server, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *mockTransferRepo) Set(name string, server *domain.Server) error {
	return nil
}

func (r *mockTransferRepo) Delete(name string) error {
	return nil
}

func (r *mockTransferRepo) Move(from, to string) error {
	return nil
}

func (r *mockTransferRepo) Search(keyword string) (map[string]map[string]*domain.Server, error) {
	return nil, nil
}

func (r *mockTransferRepo) GetGroup(group string) (map[string]*domain.Server, error) {
	if servers, ok := r.servers[group]; ok {
		return servers, nil
	}
	return nil, nil
}

func (r *mockTransferRepo) GetChangelog(name string) ([]domain.ChangelogEntry, error) {
	return nil, nil
}

func (r *mockTransferRepo) RollbackTo(name string, version int) error {
	return nil
}

func (r *mockTransferRepo) Close() error {
	return nil
}

func TestTransferService_GetServer_NotFound(t *testing.T) {
	repo := newMockTransferRepo()
	svc := NewTransferService(nil, repo)

	_, err := svc.getServer("nonexistent.server")
	if err == nil {
		t.Error("expected error for nonexistent server")
	}
}

func TestTransferService_GetServer_Found(t *testing.T) {
	repo := newMockTransferRepo()
	svc := NewTransferService(nil, repo)

	server, err := svc.getServer("test.server1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if server == nil {
		t.Error("expected server, got nil")
	}
	if server.Host != "192.168.1.100" {
		t.Errorf("expected host 192.168.1.100, got %s", server.Host)
	}
}

func TestTransferService_ParseDirectServer(t *testing.T) {
	repo := newMockTransferRepo()
	svc := NewTransferService(nil, repo)

	server, err := svc.parseDirectServer("root@192.168.1.100")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if server.User != "root" {
		t.Errorf("expected user root, got %s", server.User)
	}
	if server.Host != "192.168.1.100" {
		t.Errorf("expected host 192.168.1.100, got %s", server.Host)
	}
}

func TestTransferService_NormalizeRemotePath(t *testing.T) {
	repo := newMockTransferRepo()
	svc := NewTransferService(nil, repo)

	result := svc.normalizeRemotePath("path\\to\\file")
	expected := "path/to/file"
	if result != expected {
		t.Errorf("normalizeRemotePath() = %s, want %s", result, expected)
	}
}

func TestTransferService_RemoteBaseName(t *testing.T) {
	repo := newMockTransferRepo()
	svc := NewTransferService(nil, repo)

	result := svc.remoteBaseName("/path/to/file.txt")
	expected := "file.txt"
	if result != expected {
		t.Errorf("remoteBaseName() = %s, want %s", result, expected)
	}
}

func TestTransferService_RemoteDirName(t *testing.T) {
	repo := newMockTransferRepo()
	svc := NewTransferService(nil, repo)

	result := svc.remoteDir("/path/to/file.txt")
	expected := "/path/to"
	if result != expected {
		t.Errorf("remoteDir() = %s, want %s", result, expected)
	}
}

func TestTransferOptions_Defaults(t *testing.T) {
	opts := TransferOptions{}

	if opts.Recursive != false {
		t.Error("expected Recursive to be false by default")
	}
	if opts.Resume != false {
		t.Error("expected Resume to be false by default")
	}
	if opts.Concurrency != 0 {
		t.Errorf("expected Concurrency to be 0, got %d", opts.Concurrency)
	}
	if opts.Progress != false {
		t.Error("expected Progress to be false by default")
	}
	if opts.VerifyChecksum != false {
		t.Error("expected VerifyChecksum to be false by default")
	}
}