package store

import (
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if store.db == nil {
		t.Error("db should not be nil")
	}
}

func TestMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM config").Scan(&count)
	if err != nil {
		t.Fatalf("Query config table failed: %v", err)
	}

	err = store.db.QueryRow("SELECT COUNT(*) FROM servers").Scan(&count)
	if err != nil {
		t.Fatalf("Query servers table failed: %v", err)
	}
}

func TestStoreLock(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Error("store should not be nil")
	}
}

func TestDbPath(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if store.dbPath != dbPath {
		t.Errorf("dbPath = %q, want %q", store.dbPath, dbPath)
	}
}

func TestCreateAndReadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	_, err = store.db.Exec("INSERT INTO config (key, value) VALUES (?, ?)", "test_key", "test_value")
	if err != nil {
		t.Fatalf("Insert config failed: %v", err)
	}

	var value string
	err = store.db.QueryRow("SELECT value FROM config WHERE key = ?", "test_key").Scan(&value)
	if err != nil {
		t.Fatalf("Query config failed: %v", err)
	}

	if value != "test_value" {
		t.Errorf("value = %q, want %q", value, "test_value")
	}
}