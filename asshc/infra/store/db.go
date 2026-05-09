package store

import (
	"database/sql"
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
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now')),
			UNIQUE(group_name, name)
		);

		CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT DEFAULT (datetime('now'))
		);
	`)

	return err
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}