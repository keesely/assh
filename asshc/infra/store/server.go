package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"assh/asshc/domain"
	"assh/log"
)

// selectCols 定义服务器表查询的基础列名，在多个查询中复用。
const selectCols = "name, group_name, host, port, user_name, password_encrypted, key_file, remark, options, version"

// List 返回所有服务器记录，按分组名 -> 服务器名的二级映射组织。
// 查询时不加筛选条件，返回完整的服务器列表。
func (s *Store) List() (map[string]map[string]*domain.Server, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]map[string]*domain.Server)

	rows, err := s.db.Query(`
		SELECT ` + selectCols + `
		FROM servers
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		server, err := s.scanServer(rows)
		if err != nil {
			return nil, err
		}

		group := server.Group
		if result[group] == nil {
			result[group] = make(map[string]*domain.Server)
		}
		result[group][server.Name] = server
	}

	return result, rows.Err()
}

// Get 根据完整名称（group.name）查询单个服务器配置。
// 如果服务器不存在，返回 domain.ErrNotFound。
func (s *Store) Get(name string) (*domain.Server, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	group, serverName := domain.ParseName(rowName(name))

	row := s.db.QueryRow(`
		SELECT `+selectCols+`
		FROM servers
		WHERE group_name = ? AND name = ?
	`, group, serverName)

	server, err := s.scanServer(row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	return server, err
}

// Set 保存或更新服务器配置（upsert 语义）。
// 如果是新增记录，change_type 为 "create"，version 从 1 开始；
// 如果是更新记录，change_type 为 "update"，version 自动递增。
// 密码使用 AES-GCM 加密后存储，同时记录全量快照到变更日志。
// 整个操作在一个数据库事务中完成，保证原子性。
func (s *Store) Set(name string, server *domain.Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	group, serverName := domain.ParseName(rowName(name))
	server.Group = group
	server.Name = serverName

	// 判断变更类型并递增版本号
	changeType := domain.ChangeTypeCreate
	newVersion := 1
	existingVersion, err := s.getVersionLocked(group, serverName)
	if err == nil && existingVersion > 0 {
		changeType = domain.ChangeTypeUpdate
		newVersion = existingVersion + 1
	}
	server.Version = newVersion

	passwordEncrypted := []byte{}
	if server.Auth != nil && server.Auth.Password != "" {
		encrypted, err := s.encryptPassword(server.Auth.Password)
		if err != nil {
			return err
		}
		passwordEncrypted = encrypted
	}

	var keyFile string
	if server.Auth != nil {
		keyFile = server.Auth.KeyFile
	}

	optionsJSON := "{}"
	if server.Options != nil {
		if jsonBytes, err := json.Marshal(server.Options); err == nil {
			optionsJSON = string(jsonBytes)
		}
	}

	// 序列化全量快照（密码加密存储，用于回滚恢复）
	snapshot := *server
	if snapshot.Auth != nil && snapshot.Auth.Password != "" {
		encrypted, err := s.encryptPassword(snapshot.Auth.Password)
		if err != nil {
			return fmt.Errorf("failed to encrypt snapshot password: %w", err)
		}
		snapshot.Auth.Password = string(encrypted)
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	// 事务：同时保存服务器数据和变更日志，保证原子性
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT OR REPLACE INTO servers
			(name, group_name, host, port, user_name, password_encrypted, key_file, remark, options, version, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
	`, serverName, group, server.Host, server.Port, server.User,
		passwordEncrypted, keyFile, server.Remark, optionsJSON, newVersion)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO server_changelog
			(server_name, group_name, version, change_type, snapshot, created_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
	`, serverName, group, newVersion, changeType, string(snapshotJSON))
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Delete 删除指定名称的服务器配置。
// 如果服务器不存在，返回 domain.ErrNotFound。
func (s *Store) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	group, serverName := domain.ParseName(rowName(name))

	result, err := s.db.Exec("DELETE FROM servers WHERE group_name = ? AND name = ?", group, serverName)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// Move 将服务器从 from 重命名/移动到 to。
// 支持跨分组移动（如 "group1.srv" -> "group2.new_name"）。
// 如果目标名称已存在，返回 domain.ErrExists。
func (s *Store) Move(from, to string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fromGroup, fromName := domain.ParseName(rowName(from))
	toGroup, toName := domain.ParseName(rowName(to))

	row := s.db.QueryRow("SELECT id FROM servers WHERE group_name = ? AND name = ?", fromGroup, fromName)
	var id int
	err := row.Scan(&id)
	if err == sql.ErrNoRows {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}

	checkRow := s.db.QueryRow("SELECT id FROM servers WHERE group_name = ? AND name = ?", toGroup, toName)
	var checkID int
	checkErr := checkRow.Scan(&checkID)
	if checkErr == nil {
		return domain.ErrExists
	}
	if checkErr != nil && !errors.Is(checkErr, sql.ErrNoRows) {
		return checkErr
	}

	_, err = s.db.Exec("UPDATE servers SET group_name = ?, name = ?, updated_at = datetime('now') WHERE group_name = ? AND name = ?",
		toGroup, toName, fromGroup, fromName)
	return err
}

// Search 按关键字模糊搜索服务器，匹配字段包括名称、主机地址和备注。
// 关键字模糊匹配使用 SQL LIKE 语法。
func (s *Store) Search(keyword string) (map[string]map[string]*domain.Server, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keyword = "%" + keyword + "%"

	result := make(map[string]map[string]*domain.Server)

	rows, err := s.db.Query(`
		SELECT `+selectCols+`
		FROM servers
		WHERE name LIKE ? OR host LIKE ? OR remark LIKE ?
	`, keyword, keyword, keyword)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		server, err := s.scanServer(rows)
		if err != nil {
			return nil, err
		}

		group := server.Group
		if result[group] == nil {
			result[group] = make(map[string]*domain.Server)
		}
		result[group][server.Name] = server
	}

	return result, rows.Err()
}

// GetGroup 返回指定分组下的所有服务器。
// 分组名为空字符串时返回未分组的服务器。
func (s *Store) GetGroup(group string) (map[string]*domain.Server, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*domain.Server)

	rows, err := s.db.Query(`
		SELECT `+selectCols+`
		FROM servers
		WHERE group_name = ?
	`, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		server, err := s.scanServer(rows)
		if err != nil {
			return nil, err
		}
		result[server.Name] = server
	}

	return result, rows.Err()
}

// --- 变更日志与回滚 ---

// GetChangelog 返回指定服务器的完整变更历史，按版本号升序排列。
// 每个条目包含变更类型、版本号和配置快照。
func (s *Store) GetChangelog(name string) ([]domain.ChangelogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	group, serverName := domain.ParseName(rowName(name))

	rows, err := s.db.Query(`
		SELECT id, server_name, group_name, version, change_type, snapshot, created_at
		FROM server_changelog
		WHERE server_name = ? AND group_name = ?
		ORDER BY version ASC
	`, serverName, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.ChangelogEntry
	for rows.Next() {
		var e domain.ChangelogEntry
		var snapshotStr string
		err := rows.Scan(&e.ID, &e.ServerName, &e.GroupName, &e.Version,
			&e.ChangeType, &snapshotStr, &e.CreatedAt)
		if err != nil {
			return nil, err
		}
		// 反序列化快照
		var server domain.Server
		if err := json.Unmarshal([]byte(snapshotStr), &server); err == nil {
			e.Snapshot = &server
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return nil, domain.ErrNotFound
	}
	return entries, nil
}

// RollbackTo 将服务器配置回滚到指定的历史版本。
// 从变更日志中读取目标版本的快照，重新加密密码后写入服务器表，
// 同时在变更日志中记录一条 "rollback" 类型的变更条目。
func (s *Store) RollbackTo(name string, version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	group, serverName := domain.ParseName(rowName(name))

	if version < 1 {
		return domain.ErrInvalidVersion
	}

	// 从变更日志读取目标版本的快照
	var snapshotStr string
	err := s.db.QueryRow(`
		SELECT snapshot FROM server_changelog
		WHERE server_name = ? AND group_name = ? AND version = ?
	`, serverName, group, version).Scan(&snapshotStr)
	if err == sql.ErrNoRows {
		return domain.ErrVersionNotFound
	}
	if err != nil {
		return err
	}

	// 反序列化快照（密码已加密，需要解密后再使用）
	var restored domain.Server
	if err := json.Unmarshal([]byte(snapshotStr), &restored); err != nil {
		return err
	}
	// 解密快照中的密码
	if restored.Auth != nil && restored.Auth.Password != "" {
		decrypted, err := s.decryptPassword([]byte(restored.Auth.Password))
		if err != nil {
			return fmt.Errorf("failed to decrypt snapshot password: %w", err)
		}
		restored.Auth.Password = decrypted
	}
	restored.Group = group
	restored.Name = serverName

	// 版本号递增（从当前最新版本+1，不算回退）
	currentVersion, err := s.getVersionLocked(group, serverName)
	if err != nil {
		// 如果服务器已被删除，从快照版本开始计数
		currentVersion = version
	}
	newVersion := currentVersion + 1
	restored.Version = newVersion

	// 重新加密密码
	passwordEncrypted := []byte{}
	if restored.Auth != nil && restored.Auth.Password != "" {
		encrypted, err := s.encryptPassword(restored.Auth.Password)
		if err != nil {
			return err
		}
		passwordEncrypted = encrypted
	}

	var keyFile string
	if restored.Auth != nil {
		keyFile = restored.Auth.KeyFile
	}

	optionsJSON := "{}"
	if restored.Options != nil {
		if jsonBytes, err := json.Marshal(restored.Options); err == nil {
			optionsJSON = string(jsonBytes)
		}
	}

	// 为回滚条目序列化快照
	snapshotJSON, err := json.Marshal(restored)
	if err != nil {
		return err
	}

	// 事务：更新服务器 + 记录回滚日志
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT OR REPLACE INTO servers
			(name, group_name, host, port, user_name, password_encrypted, key_file, remark, options, version, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
	`, serverName, group, restored.Host, restored.Port, restored.User,
		passwordEncrypted, keyFile, restored.Remark, optionsJSON, newVersion)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO server_changelog
			(server_name, group_name, version, change_type, snapshot, created_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
	`, serverName, group, newVersion, domain.ChangeTypeRollback, string(snapshotJSON))
	if err != nil {
		return err
	}

	return tx.Commit()
}

// getVersionLocked 返回服务器当前的版本号。
// 调用方必须持有 s.mu 锁（读锁或写锁）。
func (s *Store) getVersionLocked(group, serverName string) (int, error) {
	var version int
	err := s.db.QueryRow(`
		SELECT version FROM servers WHERE group_name = ? AND name = ?
	`, group, serverName).Scan(&version)
	if err == sql.ErrNoRows {
		return 0, domain.ErrNotFound
	}
	return version, err
}

// --- 辅助方法 ---

// scanServer 从数据库行扫描器（Row 或 Rows）中读取并构造 Server 对象。
// 自动处理加密密码的解密和 options JSON 的反序列化。
// scanner 参数兼容 sql.Row 和 sql.Rows 类型。
func (s *Store) scanServer(scanner interface{ Scan(dest ...interface{}) error }) (*domain.Server, error) {
	var name, group, host, user, keyFile, remark string
	var port, version int
	var passwordEncrypted []byte
	var optionsJSON string

	err := scanner.Scan(&name, &group, &host, &port, &user, &passwordEncrypted, &keyFile, &remark, &optionsJSON, &version)
	if err != nil {
		return nil, err
	}

	server := &domain.Server{
		Name:    name,
		Group:   group,
		Host:    host,
		Port:    port,
		User:    user,
		Remark:  remark,
		Options: make(map[string]interface{}),
		Version: version,
	}

	if passwordEncrypted != nil && len(passwordEncrypted) > 0 {
		decrypted, err := s.decryptPassword(passwordEncrypted)
		if err == nil {
			server.Auth = &domain.Auth{Password: decrypted, KeyFile: keyFile}
		}
	} else if keyFile != "" {
		server.Auth = &domain.Auth{KeyFile: keyFile}
	}

	if optionsJSON != "" && optionsJSON != "{}" {
		if err := json.Unmarshal([]byte(optionsJSON), &server.Options); err != nil {
			log.Warnf("failed to parse options for server %s.%s: %v", group, name, err)
		}
	}

	return server, nil
}

// rowName 对名称进行预处理，当前为恒等转换，保留后续扩展可能。
func rowName(name string) string {
	if name == "" {
		return ""
	}
	return name
}
