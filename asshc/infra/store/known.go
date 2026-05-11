package store

import (
	"database/sql"

	"assh/asshc/domain"
)

// RecordDirectConnect 记录或更新一次直连操作。
// 如果同一 ID 已存在，递增 connect_count 并更新 last_connected_at；
// 如果不存在，创建新记录。
func (s *Store) RecordDirectConnect(ks *domain.KnownServer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var existingCount int
	err := s.db.QueryRow("SELECT connect_count FROM known_servers WHERE id = ?", ks.ID).Scan(&existingCount)

	if err == sql.ErrNoRows {
		_, err = s.db.Exec(`
			INSERT INTO known_servers (id, host, port, user_name, auth_fingerprint, key_backup_path, last_connected_at, connect_count, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, datetime('now'), 1, datetime('now'), datetime('now'))
		`, ks.ID, ks.Host, ks.Port, ks.User, ks.AuthFingerprint, ks.KeyBackupPath)
		return err
	}
	if err != nil {
		return err
	}

	newCount := existingCount + 1
	_, err = s.db.Exec(`
		UPDATE known_servers SET connect_count = ?, last_connected_at = datetime('now'), updated_at = datetime('now'), key_backup_path = ?
		WHERE id = ?
	`, newCount, ks.KeyBackupPath, ks.ID)
	return err
}

// LookupKnownServer 根据 ID 查找已知服务器记录。
// 如果记录不存在，返回 domain.ErrKnownServerNotFound。
func (s *Store) LookupKnownServer(id string) (*domain.KnownServer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ks, err := s.scanKnownServer(s.db.QueryRow(`
		SELECT id, host, port, user_name, auth_fingerprint, key_backup_path, last_connected_at, connect_count, created_at, updated_at
		FROM known_servers WHERE id = ?
	`, id))
	if err == sql.ErrNoRows {
		return nil, domain.ErrKnownServerNotFound
	}
	return ks, err
}

// LookupKnownServerByAuth 根据主机+端口+用户+认证指纹查找记录。
// 内部计算 ID 后调用 LookupKnownServer。
func (s *Store) LookupKnownServerByAuth(user, host string, port int, authFingerprint string) (*domain.KnownServer, error) {
	id := domain.ComputeKnownServerID(user, host, port, authFingerprint)
	return s.LookupKnownServer(id)
}

// UpdateKeyBackupPath 更新已知服务器的密钥备份路径。
func (s *Store) UpdateKeyBackupPath(id, keyBackupPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`
		UPDATE known_servers SET key_backup_path = ?, updated_at = datetime('now')
		WHERE id = ?
	`, keyBackupPath, id)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrKnownServerNotFound
	}
	return nil
}

// DeleteKnownServer 删除指定 ID 的已知服务器记录。
// 如果记录不存在，返回 domain.ErrKnownServerNotFound。
func (s *Store) DeleteKnownServer(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec("DELETE FROM known_servers WHERE id = ?", id)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrKnownServerNotFound
	}
	return nil
}

// ListKnownServers 返回所有已知服务器记录。
func (s *Store) ListKnownServers() ([]*domain.KnownServer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, host, port, user_name, auth_fingerprint, key_backup_path, last_connected_at, connect_count, created_at, updated_at
		FROM known_servers
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*domain.KnownServer
	for rows.Next() {
		ks, err := s.scanKnownServer(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, ks)
	}
	return results, rows.Err()
}

// scanKnownServer 从数据库行扫描器中读取并构造 KnownServer 对象。
func (s *Store) scanKnownServer(scanner interface{ Scan(dest ...interface{}) error }) (*domain.KnownServer, error) {
	var ks domain.KnownServer
	err := scanner.Scan(&ks.ID, &ks.Host, &ks.Port, &ks.User, &ks.AuthFingerprint, &ks.KeyBackupPath, &ks.LastConnectedAt, &ks.ConnectCount, &ks.CreatedAt, &ks.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ks, nil
}