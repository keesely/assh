package store

import (
	"testing"

	"assh/asshc/domain"
)

func TestListServers(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	servers, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if servers == nil {
		t.Error("servers should not be nil")
	}
}

func TestSetAndGetServer(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	server := &domain.Server{
		Name:   "web-01",
		Group:  "prod",
		Host:   "192.168.1.1",
		Port:   22,
		User:   "root",
		Auth:   &domain.Auth{Password: "secret"},
		Remark: "Production web server",
	}

	err = store.Set("prod.web-01", server)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	retrieved, err := store.Get("prod.web-01")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("retrieved server should not be nil")
	}

	if retrieved.Name != "web-01" {
		t.Errorf("Name = %q, want %q", retrieved.Name, "web-01")
	}
	if retrieved.Host != "192.168.1.1" {
		t.Errorf("Host = %q, want %q", retrieved.Host, "192.168.1.1")
	}
}

func TestDeleteServer(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	server := &domain.Server{
		Name: "test",
		Host: "127.0.0.1",
		Port: 22,
		User: "root",
	}

	err = store.Set("test.server", server)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	err = store.Delete("test.server")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get("test.server")
	if err == nil {
		t.Error("should return error for deleted server")
	}
}

func TestMoveServer(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	server := &domain.Server{
		Name: "old-name",
		Host: "127.0.0.1",
		Port: 22,
		User: "root",
	}

	err = store.Set("old.server", server)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	err = store.Move("old.server", "new.server")
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	_, err = store.Get("old.server")
	if err == nil {
		t.Error("old server should not exist after move")
	}

	_, err = store.Get("new.server")
	if err != nil {
		t.Fatalf("new server should exist: %v", err)
	}
}

func TestSearchServers(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	servers := []*domain.Server{
		{Name: "web-01", Group: "prod", Host: "192.168.1.1", Port: 22, User: "root"},
		{Name: "db-01", Group: "prod", Host: "192.168.1.2", Port: 22, User: "root"},
		{Name: "api-01", Group: "staging", Host: "10.0.0.1", Port: 22, User: "admin"},
	}

	for i, s := range servers {
		groupName := s.Group
		serverName := groupName + "." + s.Name
		err := store.Set(serverName, s)
		if err != nil {
			t.Fatalf("Set server %d failed: %v", i, err)
		}
	}

	results, err := store.Search("web")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("should find at least one result")
	}
}

func TestGetGroup(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	servers := []*domain.Server{
		{Name: "web-01", Host: "192.168.1.1", Port: 22, User: "root"},
		{Name: "web-02", Host: "192.168.1.2", Port: 22, User: "root"},
		{Name: "db-01", Host: "192.168.2.1", Port: 22, User: "root"},
	}

	for _, s := range servers {
		err := store.Set(s.Name, s)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	prodGroup, err := store.GetGroup("")
	if err != nil {
		t.Fatalf("GetGroup failed: %v", err)
	}

	if len(prodGroup) != 3 {
		t.Errorf("default group should have 3 servers, got %d", len(prodGroup))
	}
}

func TestServerWithOptions(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	options := map[string]interface{}{
		"keepalive": 30,
		"timeout":   10,
	}

	server := &domain.Server{
		Name:    "test",
		Host:    "127.0.0.1",
		Port:    22,
		User:    "root",
		Options: options,
	}

	err = store.Set("test.server", server)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	retrieved, err := store.Get("test.server")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Options == nil {
		t.Fatal("options should not be nil")
	}
}