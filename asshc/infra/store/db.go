// Package store 提供基于 SQLite 的数据持久化实现。
//
// 实现 port.ServerRepository 接口，使用 modernc.org/sqlite（纯 Go，
// 无 CGO 依赖）作为存储引擎。密码使用 AES-GCM 加密后存储，
// 加密密钥自动生成并保存在 config 表中。
package store

import (
	"database/sql"
	"fmt"
	"sync"

	_ "modernc.org/sqlite"
)

// allowedTables 白名单：防止 PRAGMA table_info SQL 注入。
var allowedTables = map[string]bool{
	"servers":          true,
	"cloud_accounts":   true,
	"jump_history":     true,
	"config":           true,
	"server_changelog": true,
}

// Store 是 SQLite 存储的核心结构，实现 ServerRepository 接口。
// 管理数据库连接、读写锁、以及 AES 加密密钥。
type Store struct {
	dbPath    string      // 数据库文件路径
	db        *sql.DB     // SQLite 数据库连接
	mu        sync.RWMutex // 读写锁，保证并发安全
	cryptoKey []byte      // AES-GCM 加密密钥，首次使用时自动生成
}

// NewStore 创建并初始化 SQLite 存储实例。
// 自动执行数据库迁移（建表），并从 config 表中加载加密密钥。
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

// migrate 执行数据库结构初始化与迁移。
// 自动创建 servers（服务器表）、config（配置表）和 server_changelog（变更日志表），
// 并对旧版本数据库执行字段补充（如 version 列）。
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

		CREATE TABLE IF NOT EXISTS known_servers (
			id                TEXT PRIMARY KEY,
			host              TEXT NOT NULL,
			port              INTEGER NOT NULL DEFAULT 22,
			user_name         TEXT NOT NULL DEFAULT 'root',
			auth_fingerprint  TEXT NOT NULL DEFAULT '',
			key_backup_path   TEXT DEFAULT '',
			last_connected_at TEXT,
			connect_count     INTEGER DEFAULT 1,
			created_at        TEXT DEFAULT (datetime('now')),
			updated_at        TEXT DEFAULT (datetime('now'))
		);

		CREATE INDEX IF NOT EXISTS idx_known_servers_host_port ON known_servers(host, port);

		CREATE TABLE IF NOT EXISTS cloud_accounts (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT NOT NULL DEFAULT 'default',
			type        TEXT NOT NULL DEFAULT 'qiniu',
			access_key  TEXT NOT NULL,
			secret_key  BLOB NOT NULL,
			bucket      TEXT NOT NULL,
			zone        TEXT NOT NULL DEFAULT 'huadong',
			enabled     INTEGER NOT NULL DEFAULT 1,
			created_at  TEXT DEFAULT (datetime('now')),
			updated_at  TEXT DEFAULT (datetime('now')),
			UNIQUE(name)
		);

		CREATE TABLE IF NOT EXISTS sync_history (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			direction   TEXT NOT NULL,
			status      TEXT NOT NULL,
			message     TEXT,
			pushed      INTEGER DEFAULT 0,
			updated     INTEGER DEFAULT 0,
			conflicts   INTEGER DEFAULT 0,
			timestamp   TEXT DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS jump_history (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			target_expr      TEXT NOT NULL,
			path_text        TEXT NOT NULL,
			path_data        BLOB,
			chain_count      INTEGER NOT NULL DEFAULT 0,
			last_used        TEXT,
			use_count        INTEGER DEFAULT 1,
			created_at       TEXT DEFAULT (datetime('now')),
			updated_at       TEXT DEFAULT (datetime('now'))
		);

		CREATE INDEX IF NOT EXISTS idx_jump_history_last_used ON jump_history(last_used DESC);
	`)

	if err != nil {
		return err
	}

	// 旧版本兼容迁移：为 v2.0.0-phase-4 前创建的数据库添加 version 列
	if colExists, _ := s.columnExists("servers", "version"); !colExists {
		if _, err := s.db.Exec(`ALTER TABLE servers ADD COLUMN version INTEGER NOT NULL DEFAULT 1`); err != nil {
			return fmt.Errorf("add version column: %w", err)
		}
	}

	return nil
}

// columnExists 检查指定表中是否存在指定列。
// 使用 PRAGMA table_info 查询表结构信息。
func (s *Store) columnExists(table, column string) (bool, error) {
	if !allowedTables[table] {
		return false, fmt.Errorf("table %q is not allowed", table)
	}
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

// Close 关闭数据库连接，释放资源。
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// JumpRecorder 返回跳板历史记录器。
func (s *Store) JumpRecorder() *JumpRecorder {
	return NewJumpRecorder(s)
}