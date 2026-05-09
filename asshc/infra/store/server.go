package store

import (
	"database/sql"
	"encoding/json"
	"errors"

	"assh/asshc/domain"
)

const selectCols = "name, group_name, host, port, user_name, password_encrypted, key_file, remark, options, version"

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

func (s *Store) Set(name string, server *domain.Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	group, serverName := domain.ParseName(rowName(name))
	server.Group = group
	server.Name = serverName

	// Determine change type and bump version
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

	// Snapshot: serialize full server (password in plaintext for restore)
	snapshotJSON, err := json.Marshal(server)
	if err != nil {
		return err
	}

	// Transaction: save server + log changelog atomically
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

// --- Changelog & Rollback ---

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
		// Deserialize snapshot
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

func (s *Store) RollbackTo(name string, version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	group, serverName := domain.ParseName(rowName(name))

	if version < 1 {
		return domain.ErrInvalidVersion
	}

	// Read snapshot at target version
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

	// Deserialize snapshot
	var restored domain.Server
	if err := json.Unmarshal([]byte(snapshotStr), &restored); err != nil {
		return err
	}
	restored.Group = group
	restored.Name = serverName

	// Bump version
	currentVersion, err := s.getVersionLocked(group, serverName)
	if err != nil {
		// If server was deleted, currentVersion stays 0, start from snapshot version
		currentVersion = version
	}
	newVersion := currentVersion + 1
	restored.Version = newVersion

	// Re-encrypt password if present
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

	// Snapshot for rollback entry
	snapshotJSON, err := json.Marshal(restored)
	if err != nil {
		return err
	}

	// Transaction: update server + log rollback
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

// getVersionLocked returns the current version of a server (caller must hold s.mu).
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

// --- helpers ---

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
		json.Unmarshal([]byte(optionsJSON), &server.Options)
	}

	return server, nil
}

func rowName(name string) string {
	if name == "" {
		return ""
	}
	return name
}
