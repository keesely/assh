package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"assh/asshc/domain"
)

func newTestStoreForKnown(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_known.db")
	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRecordDirectConnect_NewRecord(t *testing.T) {
	s := newTestStoreForKnown(t)

	ks := &domain.KnownServer{
		ID:              domain.ComputeKnownServerID("root", "10.0.0.1", 22, domain.ComputeAuthFingerprint("pass123", "")),
		Host:            "10.0.0.1",
		Port:            22,
		User:            "root",
		AuthFingerprint: domain.ComputeAuthFingerprint("pass123", ""),
	}

	if err := s.RecordDirectConnect(ks); err != nil {
		t.Fatalf("RecordDirectConnect failed: %v", err)
	}

	result, err := s.LookupKnownServer(ks.ID)
	if err != nil {
		t.Fatalf("LookupKnownServer failed: %v", err)
	}
	if result.ConnectCount != 1 {
		t.Errorf("expected connect_count=1, got %d", result.ConnectCount)
	}
	if result.Host != "10.0.0.1" {
		t.Errorf("expected host=10.0.0.1, got %s", result.Host)
	}
	if result.User != "root" {
		t.Errorf("expected user=root, got %s", result.User)
	}
}

func TestRecordDirectConnect_ExistingRecord(t *testing.T) {
	s := newTestStoreForKnown(t)

	ks := &domain.KnownServer{
		ID:              domain.ComputeKnownServerID("root", "10.0.0.1", 22, domain.ComputeAuthFingerprint("pass123", "")),
		Host:            "10.0.0.1",
		Port:            22,
		User:            "root",
		AuthFingerprint: domain.ComputeAuthFingerprint("pass123", ""),
	}

	if err := s.RecordDirectConnect(ks); err != nil {
		t.Fatalf("first RecordDirectConnect failed: %v", err)
	}
	if err := s.RecordDirectConnect(ks); err != nil {
		t.Fatalf("second RecordDirectConnect failed: %v", err)
	}

	result, err := s.LookupKnownServer(ks.ID)
	if err != nil {
		t.Fatalf("LookupKnownServer failed: %v", err)
	}
	if result.ConnectCount != 2 {
		t.Errorf("expected connect_count=2, got %d", result.ConnectCount)
	}
	if result.LastConnectedAt == "" {
		t.Errorf("expected last_connected_at to be set")
	}
}

func TestLookupKnownServer(t *testing.T) {
	s := newTestStoreForKnown(t)

	ks := &domain.KnownServer{
		ID:              domain.ComputeKnownServerID("admin", "192.168.1.1", 2222, domain.ComputeAuthFingerprint("", "/home/admin/.ssh/id_rsa")),
		Host:            "192.168.1.1",
		Port:            2222,
		User:            "admin",
		AuthFingerprint: domain.ComputeAuthFingerprint("", "/home/admin/.ssh/id_rsa"),
	}

	if err := s.RecordDirectConnect(ks); err != nil {
		t.Fatalf("RecordDirectConnect failed: %v", err)
	}

	result, err := s.LookupKnownServer(ks.ID)
	if err != nil {
		t.Fatalf("LookupKnownServer failed: %v", err)
	}
	if result.Port != 2222 {
		t.Errorf("expected port=2222, got %d", result.Port)
	}

	_, err = s.LookupKnownServer("nonexistent_id")
	if err != domain.ErrKnownServerNotFound {
		t.Errorf("expected ErrKnownServerNotFound, got %v", err)
	}
}

func TestLookupKnownServerByAuth(t *testing.T) {
	s := newTestStoreForKnown(t)

	authFP := domain.ComputeAuthFingerprint("pass123", "")
	ks := &domain.KnownServer{
		ID:              domain.ComputeKnownServerID("root", "10.0.0.1", 22, authFP),
		Host:            "10.0.0.1",
		Port:            22,
		User:            "root",
		AuthFingerprint: authFP,
	}

	if err := s.RecordDirectConnect(ks); err != nil {
		t.Fatalf("RecordDirectConnect failed: %v", err)
	}

	result, err := s.LookupKnownServerByAuth("root", "10.0.0.1", 22, authFP)
	if err != nil {
		t.Fatalf("LookupKnownServerByAuth failed: %v", err)
	}
	if result.ID != ks.ID {
		t.Errorf("expected id=%s, got %s", ks.ID, result.ID)
	}
}

func TestUpdateKeyBackupPath(t *testing.T) {
	s := newTestStoreForKnown(t)

	ks := &domain.KnownServer{
		ID:              domain.ComputeKnownServerID("root", "10.0.0.1", 22, ""),
		Host:            "10.0.0.1",
		Port:            22,
		User:            "root",
		AuthFingerprint: "",
	}

	if err := s.RecordDirectConnect(ks); err != nil {
		t.Fatalf("RecordDirectConnect failed: %v", err)
	}

	if err := s.UpdateKeyBackupPath(ks.ID, "data/keys/10.0.0.1_root.key"); err != nil {
		t.Fatalf("UpdateKeyBackupPath failed: %v", err)
	}

	result, err := s.LookupKnownServer(ks.ID)
	if err != nil {
		t.Fatalf("LookupKnownServer failed: %v", err)
	}
	if result.KeyBackupPath != "data/keys/10.0.0.1_root.key" {
		t.Errorf("expected key_backup_path=data/keys/10.0.0.1_root.key, got %s", result.KeyBackupPath)
	}

	err = s.UpdateKeyBackupPath("nonexistent_id", "path")
	if err != domain.ErrKnownServerNotFound {
		t.Errorf("expected ErrKnownServerNotFound, got %v", err)
	}
}

func TestDeleteKnownServer(t *testing.T) {
	s := newTestStoreForKnown(t)

	ks := &domain.KnownServer{
		ID:              domain.ComputeKnownServerID("root", "10.0.0.1", 22, ""),
		Host:            "10.0.0.1",
		Port:            22,
		User:            "root",
		AuthFingerprint: "",
	}

	if err := s.RecordDirectConnect(ks); err != nil {
		t.Fatalf("RecordDirectConnect failed: %v", err)
	}

	if err := s.DeleteKnownServer(ks.ID); err != nil {
		t.Fatalf("DeleteKnownServer failed: %v", err)
	}

	_, err := s.LookupKnownServer(ks.ID)
	if err != domain.ErrKnownServerNotFound {
		t.Errorf("expected ErrKnownServerNotFound after delete, got %v", err)
	}

	err = s.DeleteKnownServer("nonexistent_id")
	if err != domain.ErrKnownServerNotFound {
		t.Errorf("expected ErrKnownServerNotFound for nonexistent, got %v", err)
	}
}

func TestListKnownServers(t *testing.T) {
	s := newTestStoreForKnown(t)

	servers := []*domain.KnownServer{
		{
			ID:              domain.ComputeKnownServerID("root", "10.0.0.1", 22, domain.ComputeAuthFingerprint("pw1", "")),
			Host:            "10.0.0.1",
			Port:            22,
			User:            "root",
			AuthFingerprint: domain.ComputeAuthFingerprint("pw1", ""),
		},
		{
			ID:              domain.ComputeKnownServerID("admin", "192.168.1.1", 2222, domain.ComputeAuthFingerprint("", "/home/admin/.ssh/id_rsa")),
			Host:            "192.168.1.1",
			Port:            2222,
			User:            "admin",
			AuthFingerprint: domain.ComputeAuthFingerprint("", "/home/admin/.ssh/id_rsa"),
		},
	}

	for _, ks := range servers {
		if err := s.RecordDirectConnect(ks); err != nil {
			t.Fatalf("RecordDirectConnect failed: %v", err)
		}
	}

	results, err := s.ListKnownServers()
	if err != nil {
		t.Fatalf("ListKnownServers failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 known servers, got %d", len(results))
	}
}

func TestComputeKnownServerID(t *testing.T) {
	id1 := domain.ComputeKnownServerID("root", "10.0.0.1", 22, "fingerprint1")
	id2 := domain.ComputeKnownServerID("root", "10.0.0.1", 22, "fingerprint1")
	if id1 != id2 {
		t.Errorf("same inputs should produce same ID")
	}

	id3 := domain.ComputeKnownServerID("root", "10.0.0.1", 22, "fingerprint2")
	if id1 == id3 {
		t.Errorf("different fingerprint should produce different ID")
	}

	id4 := domain.ComputeKnownServerID("admin", "10.0.0.1", 22, "fingerprint1")
	if id1 == id4 {
		t.Errorf("different user should produce different ID")
	}

	if len(id1) != 64 {
		t.Errorf("expected SHA256 hex length of 64, got %d", len(id1))
	}
}

func TestComputeAuthFingerprint(t *testing.T) {
	pwOnly := domain.ComputeAuthFingerprint("password123", "")
	if pwOnly == "" {
		t.Errorf("password-only fingerprint should not be empty")
	}
	if len(pwOnly) != 64 {
		t.Errorf("password-only fingerprint should be SHA256 hex (64 chars), got %d", len(pwOnly))
	}

	keyOnly := domain.ComputeAuthFingerprint("", "/home/user/.ssh/id_rsa")
	if keyOnly == "" {
		t.Errorf("key-only fingerprint should not be empty")
	}
	if len(keyOnly) != 64 {
		t.Errorf("key-only fingerprint should be SHA256 hex (64 chars), got %d", len(keyOnly))
	}

	both := domain.ComputeAuthFingerprint("password123", "/home/user/.ssh/id_rsa")
	if both == "" {
		t.Errorf("combined fingerprint should not be empty")
	}
	parts := splitFingerprint(both)
	if len(parts) != 2 {
		t.Errorf("combined fingerprint should have 2 parts separated by ':', got %d parts", len(parts))
	}
	if len(parts[0]) != 64 || len(parts[1]) != 64 {
		t.Errorf("combined fingerprint parts should each be 64 chars (SHA256 hex), got %d and %d", len(parts[0]), len(parts[1]))
	}

	agentOnly := domain.ComputeAuthFingerprint("", "")
	if agentOnly != "" {
		t.Errorf("agent-only (no password, no key) fingerprint should be empty string")
	}
}

func splitFingerprint(fp string) []string {
	for i := 0; i < len(fp); i++ {
		if fp[i] == ':' {
			return []string{fp[:i], fp[i+1:]}
		}
	}
	return []string{fp}
}

func TestKnownServerDBSchema(t *testing.T) {
	s := newTestStoreForKnown(t)

	rows, err := s.db.Query("PRAGMA table_info(known_servers)")
	if err != nil {
		t.Fatalf("PRAGMA table_info failed: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		columns[name] = true
	}

	expectedColumns := []string{"id", "host", "port", "user_name", "auth_fingerprint", "key_backup_path", "last_connected_at", "connect_count", "created_at", "updated_at"}
	for _, col := range expectedColumns {
		if !columns[col] {
			t.Errorf("missing column: %s", col)
		}
	}
}