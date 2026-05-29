package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"assh/asshc/domain"
	"assh/asshc/port"
	"assh/log"
)

const sqliteTimeFormat = "2006-01-02 15:04:05"

// parseSQLiteTime 解析 SQLite datetime() 函数返回的时间字符串。
func parseSQLiteTime(s string) (time.Time, error) {
	return time.Parse(sqliteTimeFormat, s)
}

// TunnelServiceStore 实现 port.TunnelServiceRepository 接口。
type TunnelServiceStore struct {
	store *Store
}

// NewTunnelServiceStore 创建 TunnelServiceStore 实例。
func NewTunnelServiceStore(store *Store) *TunnelServiceStore {
	return &TunnelServiceStore{store: store}
}

// Create 创建新的隧道服务配置。
func (ts *TunnelServiceStore) Create(svc *domain.TunnelService) error {
	ts.store.mu.Lock()
	defer ts.store.mu.Unlock()

	forwardsJSON, err := json.Marshal(svc.Forwards)
	if err != nil {
		return fmt.Errorf("marshal forwards: %w", err)
	}

	var authEncrypted []byte
	if svc.Auth != "" {
		authEncrypted, err = ts.store.encryptData(svc.Auth)
		if err != nil {
			return fmt.Errorf("encrypt auth: %w", err)
		}
	}

	result, err := ts.store.db.Exec(`
		INSERT INTO tunnel_services
			(name, group_name, server_name, host, port, user_name, jump_chain,
			 forwards, auth_encrypted, daemon, auto_reconnect, remark, enabled,
			 use_count, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 1, datetime('now'), datetime('now'))
	`, svc.Name, svc.GroupName, svc.ServerName, svc.Host, svc.Port, svc.UserName,
		svc.JumpChain, string(forwardsJSON), authEncrypted, boolToInt(svc.Daemon),
		svc.AutoReconnect, svc.Remark, boolToInt(svc.Enabled))
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	svc.ID = id
	svc.Version = 1
	return nil
}

// Update 更新隧道服务配置。
func (ts *TunnelServiceStore) Update(svc *domain.TunnelService) error {
	ts.store.mu.Lock()
	defer ts.store.mu.Unlock()

	forwardsJSON, err := json.Marshal(svc.Forwards)
	if err != nil {
		return fmt.Errorf("marshal forwards: %w", err)
	}

	var authEncrypted []byte
	if svc.Auth != "" {
		authEncrypted, err = ts.store.encryptData(svc.Auth)
		if err != nil {
			return fmt.Errorf("encrypt auth: %w", err)
		}
	}

	result, err := ts.store.db.Exec(`
		UPDATE tunnel_services SET
			name = ?, group_name = ?, server_name = ?, host = ?, port = ?,
			user_name = ?, jump_chain = ?, forwards = ?, auth_encrypted = ?,
			daemon = ?, auto_reconnect = ?, remark = ?, enabled = ?,
			version = version + 1, updated_at = datetime('now')
		WHERE id = ?
	`, svc.Name, svc.GroupName, svc.ServerName, svc.Host, svc.Port, svc.UserName,
		svc.JumpChain, string(forwardsJSON), authEncrypted, boolToInt(svc.Daemon),
		svc.AutoReconnect, svc.Remark, boolToInt(svc.Enabled), svc.ID)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete 删除指定 ID 的隧道服务配置。
func (ts *TunnelServiceStore) Delete(id int64) error {
	ts.store.mu.Lock()
	defer ts.store.mu.Unlock()

	result, err := ts.store.db.Exec("DELETE FROM tunnel_services WHERE id = ?", id)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// GetByID 根据 ID 获取隧道服务配置。
func (ts *TunnelServiceStore) GetByID(id int64) (*domain.TunnelService, error) {
	ts.store.mu.RLock()
	defer ts.store.mu.RUnlock()

	row := ts.store.db.QueryRow(`
		SELECT id, name, group_name, server_name, host, port, user_name,
		       jump_chain, forwards, auth_encrypted, daemon, auto_reconnect,
		       remark, enabled, use_count, last_used, version, created_at, updated_at
		FROM tunnel_services WHERE id = ?
	`, id)

	return ts.scanTunnelService(row)
}

// GetByName 根据分组名和服务名获取隧道服务配置。
func (ts *TunnelServiceStore) GetByName(groupName, name string) (*domain.TunnelService, error) {
	ts.store.mu.RLock()
	defer ts.store.mu.RUnlock()

	row := ts.store.db.QueryRow(`
		SELECT id, name, group_name, server_name, host, port, user_name,
		       jump_chain, forwards, auth_encrypted, daemon, auto_reconnect,
		       remark, enabled, use_count, last_used, version, created_at, updated_at
		FROM tunnel_services WHERE group_name = ? AND name = ?
	`, groupName, name)

	return ts.scanTunnelService(row)
}

// List 返回指定分组下的所有隧道服务配置。
// groupName 为空时返回所有配置。
func (ts *TunnelServiceStore) List(groupName string) ([]*domain.TunnelService, error) {
	ts.store.mu.RLock()
	defer ts.store.mu.RUnlock()

	var rows *sql.Rows
	var err error

	if groupName == "" {
		rows, err = ts.store.db.Query(`
			SELECT id, name, group_name, server_name, host, port, user_name,
			       jump_chain, forwards, auth_encrypted, daemon, auto_reconnect,
			       remark, enabled, use_count, last_used, version, created_at, updated_at
			FROM tunnel_services ORDER BY group_name, name
		`)
	} else {
		rows, err = ts.store.db.Query(`
			SELECT id, name, group_name, server_name, host, port, user_name,
			       jump_chain, forwards, auth_encrypted, daemon, auto_reconnect,
			       remark, enabled, use_count, last_used, version, created_at, updated_at
			FROM tunnel_services WHERE group_name = ? ORDER BY name
		`, groupName)
	}
	if err != nil {
		return nil, err
	}
	defer closeAndLog(rows, "rows")

	var results []*domain.TunnelService
	for rows.Next() {
		svc, err := ts.scanTunnelServiceRows(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, svc)
	}
	return results, rows.Err()
}

// RecordUsage 记录一次使用，递增 use_count 并更新 last_used。
func (ts *TunnelServiceStore) RecordUsage(id int64) error {
	ts.store.mu.Lock()
	defer ts.store.mu.Unlock()

	_, err := ts.store.db.Exec(`
		UPDATE tunnel_services
		SET use_count = use_count + 1, last_used = datetime('now')
		WHERE id = ?
	`, id)
	return err
}

// scanTunnelService 从数据库行扫描器中读取并构造 TunnelService 对象。
func (ts *TunnelServiceStore) scanTunnelService(scanner interface{ Scan(dest ...interface{}) error }) (*domain.TunnelService, error) {
	var svc domain.TunnelService
	var forwardsJSON string
	var authEncrypted []byte
	var daemonInt, enabledInt int
	var lastUsed, createdAt, updatedAt sql.NullString

	err := scanner.Scan(
		&svc.ID, &svc.Name, &svc.GroupName, &svc.ServerName,
		&svc.Host, &svc.Port, &svc.UserName, &svc.JumpChain,
		&forwardsJSON, &authEncrypted, &daemonInt, &svc.AutoReconnect,
		&svc.Remark, &enabledInt, &svc.UseCount, &lastUsed,
		&svc.Version, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	svc.Daemon = daemonInt != 0
	svc.Enabled = enabledInt != 0

	if lastUsed.Valid {
		if t, err := parseSQLiteTime(lastUsed.String); err == nil {
			svc.LastUsed = &t
		}
	}
	if createdAt.Valid {
		if t, err := parseSQLiteTime(createdAt.String); err == nil {
			svc.CreatedAt = t
		}
	}
	if updatedAt.Valid {
		if t, err := parseSQLiteTime(updatedAt.String); err == nil {
			svc.UpdatedAt = t
		}
	}

	if forwardsJSON != "" && forwardsJSON != "[]" {
		if err := json.Unmarshal([]byte(forwardsJSON), &svc.Forwards); err != nil {
			log.Warnf("failed to parse tunnel forwards: %v", err)
		}
	}

	if len(authEncrypted) > 0 {
		decrypted, err := ts.store.decryptData(authEncrypted)
		if err == nil {
			svc.Auth = decrypted
		}
	}

	return &svc, nil
}

// scanTunnelServiceRows 从 sql.Rows 中扫描并构造 TunnelService 对象。
func (ts *TunnelServiceStore) scanTunnelServiceRows(rows *sql.Rows) (*domain.TunnelService, error) {
	return ts.scanTunnelService(rows)
}

// 编译期接口合规检查
var _ port.TunnelServiceRepository = (*TunnelServiceStore)(nil)
