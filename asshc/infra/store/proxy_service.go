package store

import (
	"database/sql"
	"time"

	"assh/asshc/domain"
	"assh/asshc/port"
)

// selectProxyCols 定义代理服务表查询的基础列名，在多个查询中复用。
const selectProxyCols = `id, name, group_name, server_name, host, port, user_name,
	jump_chain, proxy_type, socks5_addr, http_addr, tcp_addr,
	reverse, listen_type, auth_encrypted, rule_file, log_dir,
	daemon, auto_reconnect, remark, enabled, use_count, last_used,
	version, created_at, updated_at`

// ProxyStore 实现 port.ProxyServiceRepository 接口。
// 使用 Store 的 cryptoKey 进行 auth 字段的 AES-GCM 加密。
type ProxyStore struct {
	store *Store
}

// NewProxyStore 创建 ProxyStore 实例。
func NewProxyStore(store *Store) *ProxyStore {
	return &ProxyStore{store: store}
}

// 编译期接口合规检查
var _ port.ProxyServiceRepository = (*ProxyStore)(nil)

// Create 创建新的代理服务配置。
func (ps *ProxyStore) Create(svc *domain.ProxyService) error {
	ps.store.mu.Lock()
	defer ps.store.mu.Unlock()

	// 加密 auth 字段
	var authEncrypted []byte
	if svc.Auth != "" {
		encrypted, err := ps.store.encryptData(svc.Auth)
		if err != nil {
			return err
		}
		authEncrypted = encrypted
	}

	// 处理 last_used
	var lastUsed sql.NullString
	if svc.LastUsed != nil {
		lastUsed = sql.NullString{String: svc.LastUsed.Format(time.RFC3339), Valid: true}
	}

	result, err := ps.store.db.Exec(`
		INSERT INTO proxy_services
			(name, group_name, server_name, host, port, user_name,
			jump_chain, proxy_type, socks5_addr, http_addr, tcp_addr,
			reverse, listen_type, auth_encrypted, rule_file, log_dir,
			daemon, auto_reconnect, remark, enabled, use_count, last_used,
			version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, svc.Name, svc.GroupName, svc.ServerName, svc.Host, svc.Port, svc.UserName,
		svc.JumpChain, svc.ProxyType, svc.SOCKS5Addr, svc.HTTPAddr, svc.TCPAddr,
		boolToInt(svc.Reverse), svc.ListenType, authEncrypted, svc.RuleFile, svc.LogDir,
		boolToInt(svc.Daemon), svc.AutoReconnect, svc.Remark, boolToInt(svc.Enabled),
		svc.UseCount, lastUsed, svc.Version)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	svc.ID = id

	return nil
}

// Update 更新代理服务配置。
func (ps *ProxyStore) Update(svc *domain.ProxyService) error {
	ps.store.mu.Lock()
	defer ps.store.mu.Unlock()

	// 加密 auth 字段
	var authEncrypted []byte
	if svc.Auth != "" {
		encrypted, err := ps.store.encryptData(svc.Auth)
		if err != nil {
			return err
		}
		authEncrypted = encrypted
	}

	// 处理 last_used
	var lastUsed sql.NullString
	if svc.LastUsed != nil {
		lastUsed = sql.NullString{String: svc.LastUsed.Format(time.RFC3339), Valid: true}
	}

	_, err := ps.store.db.Exec(`
		UPDATE proxy_services SET
			name = ?, group_name = ?, server_name = ?, host = ?, port = ?, user_name = ?,
			jump_chain = ?, proxy_type = ?, socks5_addr = ?, http_addr = ?, tcp_addr = ?,
			reverse = ?, listen_type = ?, auth_encrypted = ?, rule_file = ?, log_dir = ?,
			daemon = ?, auto_reconnect = ?, remark = ?, enabled = ?, use_count = ?, last_used = ?,
			version = ?, updated_at = datetime('now')
		WHERE id = ?
	`, svc.Name, svc.GroupName, svc.ServerName, svc.Host, svc.Port, svc.UserName,
		svc.JumpChain, svc.ProxyType, svc.SOCKS5Addr, svc.HTTPAddr, svc.TCPAddr,
		boolToInt(svc.Reverse), svc.ListenType, authEncrypted, svc.RuleFile, svc.LogDir,
		boolToInt(svc.Daemon), svc.AutoReconnect, svc.Remark, boolToInt(svc.Enabled),
		svc.UseCount, lastUsed, svc.Version, svc.ID)

	return err
}

// Delete 删除指定 ID 的代理服务配置。
func (ps *ProxyStore) Delete(id int64) error {
	ps.store.mu.Lock()
	defer ps.store.mu.Unlock()

	result, err := ps.store.db.Exec("DELETE FROM proxy_services WHERE id = ?", id)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// GetByID 根据 ID 获取代理服务配置。
func (ps *ProxyStore) GetByID(id int64) (*domain.ProxyService, error) {
	ps.store.mu.RLock()
	defer ps.store.mu.RUnlock()

	row := ps.store.db.QueryRow(`
		SELECT `+selectProxyCols+`
		FROM proxy_services
		WHERE id = ?
	`, id)

	svc, err := ps.scanProxyService(row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	return svc, err
}

// GetByName 根据分组名和服务名获取代理服务配置。
func (ps *ProxyStore) GetByName(groupName, name string) (*domain.ProxyService, error) {
	ps.store.mu.RLock()
	defer ps.store.mu.RUnlock()

	row := ps.store.db.QueryRow(`
		SELECT `+selectProxyCols+`
		FROM proxy_services
		WHERE group_name = ? AND name = ?
	`, groupName, name)

	svc, err := ps.scanProxyService(row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	return svc, err
}

// List 返回指定分组下的所有代理服务配置。
// groupName 为空时返回所有配置。
func (ps *ProxyStore) List(groupName string) ([]*domain.ProxyService, error) {
	ps.store.mu.RLock()
	defer ps.store.mu.RUnlock()

	var rows *sql.Rows
	var err error

	if groupName == "" {
		rows, err = ps.store.db.Query(`
			SELECT ` + selectProxyCols + `
			FROM proxy_services
			ORDER BY group_name, name
		`)
	} else {
		rows, err = ps.store.db.Query(`
			SELECT `+selectProxyCols+`
			FROM proxy_services
			WHERE group_name = ?
			ORDER BY name
		`, groupName)
	}
	if err != nil {
		return nil, err
	}
	defer closeAndLog(rows, "rows")

	var results []*domain.ProxyService
	for rows.Next() {
		svc, err := ps.scanProxyService(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, svc)
	}

	return results, rows.Err()
}

// RecordUsage 记录一次使用，递增 use_count 并更新 last_used。
func (ps *ProxyStore) RecordUsage(id int64) error {
	ps.store.mu.Lock()
	defer ps.store.mu.Unlock()

	_, err := ps.store.db.Exec(`
		UPDATE proxy_services
		SET use_count = use_count + 1, last_used = datetime('now'), updated_at = datetime('now')
		WHERE id = ?
	`, id)

	return err
}

// scanProxyService 从数据库行扫描器中读取并构造 ProxyService 对象。
// 自动处理加密 auth 字段的解密和 bool 字段的转换。
func (ps *ProxyStore) scanProxyService(scanner interface{ Scan(dest ...interface{}) error }) (*domain.ProxyService, error) {
	var id int64
	var name, groupName, serverName, host, userName string
	var jumpChain, proxyType, socks5Addr, httpAddr, tcpAddr string
	var listenType, ruleFile, logDir, autoReconnect, remark string
	var port int
	var authEncrypted []byte
	var reverse, daemon, enabled int
	var useCount, version int
	var lastUsed sql.NullString
	var createdAt, updatedAt string

	err := scanner.Scan(
		&id, &name, &groupName, &serverName, &host, &port, &userName,
		&jumpChain, &proxyType, &socks5Addr, &httpAddr, &tcpAddr,
		&reverse, &listenType, &authEncrypted, &ruleFile, &logDir,
		&daemon, &autoReconnect, &remark, &enabled, &useCount, &lastUsed,
		&version, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	svc := &domain.ProxyService{
		ID:            id,
		Name:          name,
		GroupName:     groupName,
		ServerName:    serverName,
		Host:          host,
		Port:          port,
		UserName:      userName,
		JumpChain:     jumpChain,
		ProxyType:     proxyType,
		SOCKS5Addr:    socks5Addr,
		HTTPAddr:      httpAddr,
		TCPAddr:       tcpAddr,
		Reverse:       intToBool(reverse),
		ListenType:    listenType,
		RuleFile:      ruleFile,
		LogDir:        logDir,
		Daemon:        intToBool(daemon),
		AutoReconnect: autoReconnect,
		Remark:        remark,
		Enabled:       intToBool(enabled),
		UseCount:      useCount,
		Version:       version,
	}

	// 解密 auth 字段
	if len(authEncrypted) > 0 {
		decrypted, err := ps.store.decryptData(authEncrypted)
		if err == nil {
			svc.Auth = decrypted
		}
	}

	// 解析 last_used
	if lastUsed.Valid {
		t, err := time.Parse(time.RFC3339, lastUsed.String)
		if err == nil {
			svc.LastUsed = &t
		}
	}

	// 解析 created_at
	if createdAt != "" {
		t, err := time.Parse(time.RFC3339, createdAt)
		if err == nil {
			svc.CreatedAt = t
		}
	}

	// 解析 updated_at
	if updatedAt != "" {
		t, err := time.Parse(time.RFC3339, updatedAt)
		if err == nil {
			svc.UpdatedAt = t
		}
	}

	return svc, nil
}

