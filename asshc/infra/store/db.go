package store

import (
	"database/sql"
	"fmt"
	"sync"

	_ "modernc.org/sqlite"
)

type Store struct {
	dbPath    string
	db        *sql.DB
	mu        sync.RWMutex
	cryptoKey []byte
}

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	store := &Store{
		dbPath:    dbPath,
		db:        db,
		mu:        sync.RWMutex{},
		cryptoKey: nil,
	}

	if err := store.migrate(); err != nil {
		store.db.Close()
		return nil, err
	}

	if key, err := store.getCryptoKey(); err == nil && key != nil {
		store.cryptoKey = key
	}

	return store, nil
}

func (s *Store) migrate() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS servers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			group_name TEXT NOT NULL DEFAULT '',
			host TEXT NOT NULL,
			port INTEGER NOT NULL DEFAULT 22,
			user_name TEXT NOT NULL DEFAULT 'root',
			password_encrypted BLOB,
			key_file TEXT DEFAULT '',
			remark TEXT DEFAULT '',
			options TEXT DEFAULT '{}',
			version INTEGER NOT NULL DEFAULT 1,
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now')),
			UNIQUE(group_name, name)
		);

		CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS server_changelog (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			server_name TEXT NOT NULL,
			group_name TEXT NOT NULL DEFAULT '',
			version INTEGER NOT NULL,
			change_type TEXT NOT NULL,
			snapshot TEXT NOT NULL,
			created_at TEXT DEFAULT (datetime('now'))
		);

		CREATE INDEX IF NOT EXISTS idx_changelog_lookup ON server_changelog(server_name, group_name);
		CREATE INDEX IF NOT EXISTS idx_changelog_version ON server_changelog(server_name, group_name, version);
	`)

	if err != nil {
		return err
	}

	// Migration: add version column for databases created before v2.0.0-phase-4+
	if colExists, _ := s.columnExists("servers", "version"); !colExists {
		if _, err := s.db.Exec(`ALTER TABLE servers ADD COLUMN version INTEGER NOT NULL DEFAULT 1`); err != nil {
			return fmt.Errorf("add version column: %w", err)
		}
	}

	return nil
}

func (s *Store) columnExists(table, column string) (bool, error) {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}