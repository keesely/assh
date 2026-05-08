package store

import (
	"database/sql"
	"encoding/json"
	"errors"

	"assh/asshc/domain"
)

func (s *Store) List() (map[string]map[string]*domain.Server, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]map[string]*domain.Server)

	rows, err := s.db.Query(`
		SELECT name, group_name, host, port, user_name, password_encrypted, key_file, remark, options
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
		SELECT name, group_name, host, port, user_name, password_encrypted, key_file, remark, options
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

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO servers (name, group_name, host, port, user_name, password_encrypted, key_file, remark, options, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
	`, serverName, group, server.Host, server.Port, server.User, passwordEncrypted, keyFile, server.Remark, optionsJSON)

	return err
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

	_, err = s.db.Exec("UPDATE servers SET group_name = ?, name = ?, updated_at = datetime('now') WHERE group_name = ? AND name = ?", toGroup, toName, fromGroup, fromName)
	return err
}

func (s *Store) Search(keyword string) (map[string]map[string]*domain.Server, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keyword = "%" + keyword + "%"

	result := make(map[string]map[string]*domain.Server)

	rows, err := s.db.Query(`
		SELECT name, group_name, host, port, user_name, password_encrypted, key_file, remark, options
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
		SELECT name, group_name, host, port, user_name, password_encrypted, key_file, remark, options
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

func (s *Store) scanServer(scanner interface{ Scan(dest ...interface{}) error }) (*domain.Server, error) {
	var name, group, host, user, keyFile, remark string
	var port int
	var passwordEncrypted []byte
	var optionsJSON string

	err := scanner.Scan(&name, &group, &host, &port, &user, &passwordEncrypted, &keyFile, &remark, &optionsJSON)
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