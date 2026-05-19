package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"assh/asshc/domain"
	"assh/asshc/infra/crypto"
	"assh/log"
)

// accountSelectCols 定义云账户表查询的基础列名。
const accountSelectCols = "id, name, type, access_key, secret_key, bucket, zone, enabled, created_at, updated_at"

// GetAccount 获取默认云账户信息。
// 如果不存在返回 domain.ErrAccountNotFound。
func (s *Store) GetAccount() (*domain.CloudAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(`
		SELECT ` + accountSelectCols + `
		FROM cloud_accounts
		WHERE name = 'default'
	`)

	return s.scanAccount(row)
}

// ListAccounts 返回所有云账户列表。
func (s *Store) ListAccounts() ([]*domain.CloudAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT ` + accountSelectCols + `
		FROM cloud_accounts
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*domain.CloudAccount
	for rows.Next() {
		acct, err := s.scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, acct)
	}
	return accounts, rows.Err()
}

// SetAccount 保存云账户信息（upsert）。
// 自动加密 SecretKey 后存储。
func (s *Store) SetAccount(acct *domain.CloudAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 加密 SecretKey
	encryptedSecret, err := s.encryptData(acct.SecretKey)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.Exec(`
		INSERT INTO cloud_accounts (name, type, access_key, secret_key, bucket, zone, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			type = COALESCE(excluded.type, type),
			access_key = COALESCE(excluded.access_key, access_key),
			secret_key = COALESCE(excluded.secret_key, secret_key),
			bucket = COALESCE(excluded.bucket, bucket),
			zone = COALESCE(excluded.zone, zone),
			enabled = COALESCE(excluded.enabled, enabled),
			updated_at = ?
	`, "default", acct.Type, acct.AccessKey, encryptedSecret, acct.Bucket, acct.Zone, boolToInt(acct.Enabled), now, now)

	return err
}

// DeleteAccount 删除默认云账户。
// 如果不存在返回 domain.ErrAccountNotFound。
func (s *Store) DeleteAccount() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec("DELETE FROM cloud_accounts WHERE name = 'default'")
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrAccountNotFound
	}
	return nil
}

// TestAccount 测试云账户连接是否正常（验证凭据有效）。
// 返回 nil 表示连接正常。
func (s *Store) TestAccount() error {
	// 验证账户是否存在且包含有效凭据
	acct, err := s.GetAccount()
	if err != nil {
		return err
	}

	if acct.AccessKey == "" || acct.SecretKey == "" {
		return errors.New("cloud account credentials are incomplete")
	}

	return nil
}

// RecordSyncHistory 记录同步历史。
func (s *Store) RecordSyncHistory(history *domain.SyncHistory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO sync_history (direction, status, message, pushed, updated, conflicts, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'))
	`, history.Direction, history.Status, history.Message, history.Pushed, history.Updated, history.Conflicts)

	return err
}

// GetSyncHistory 获取同步历史记录列表。
// limit 限制返回条数，0 表示不限。
func (s *Store) GetSyncHistory(limit int) ([]*domain.SyncHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var query string
	if limit > 0 {
		query = `
			SELECT id, direction, status, message, pushed, updated, conflicts, timestamp
			FROM sync_history
			ORDER BY id DESC
			LIMIT ?
		`
	} else {
		query = `
			SELECT id, direction, status, message, pushed, updated, conflicts, timestamp
			FROM sync_history
			ORDER BY id DESC
		`
	}

	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = s.db.Query(query, limit)
	} else {
		rows, err = s.db.Query(query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []*domain.SyncHistory
	for rows.Next() {
		h, err := s.scanSyncHistory(rows)
		if err != nil {
			return nil, err
		}
		histories = append(histories, h)
	}
	return histories, rows.Err()
}

// scanAccount 从行扫描器中读取并构造 CloudAccount 对象。
func (s *Store) scanAccount(scanner interface{ Scan(dest ...interface{}) error }) (*domain.CloudAccount, error) {
	var id int
	var name, acctType, accessKey, bucket, zone, createdAt, updatedAt string
	var secretEncrypted []byte
	var enabled int

	err := scanner.Scan(&id, &name, &acctType, &accessKey, &secretEncrypted, &bucket, &zone, &enabled, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrAccountNotFound
	}
	if err != nil {
		return nil, err
	}

	// 解密 SecretKey
	secretKey, err := s.decryptData(secretEncrypted)
	if err != nil {
		return nil, err
	}

	acct := &domain.CloudAccount{
		ID:        id,
		Name:      name,
		Type:      acctType,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    bucket,
		Zone:      zone,
		Enabled:   intToBool(enabled),
	}
	var parseErr error
	acct.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAt)
	if parseErr != nil {
		log.Warnf("failed to parse created_at for account %q: %v", name, parseErr)
		acct.CreatedAt = time.Time{}
	}
	acct.UpdatedAt, parseErr = time.Parse(time.RFC3339, updatedAt)
	if parseErr != nil {
		log.Warnf("failed to parse updated_at for account %q: %v", name, parseErr)
		acct.UpdatedAt = time.Time{}
	}

	return acct, nil
}

// scanSyncHistory 从行扫描器中读取并构造 SyncHistory 对象。
func (s *Store) scanSyncHistory(scanner interface{ Scan(dest ...interface{}) error }) (*domain.SyncHistory, error) {
	var id int
	var direction, status, message, ts string
	var pushed, updated, conflicts int

	err := scanner.Scan(&id, &direction, &status, &message, &pushed, &updated, &conflicts, &ts)
	if err != nil {
		return nil, err
	}

	t, _ := time.Parse(time.RFC3339, ts)
	return &domain.SyncHistory{
		ID:        id,
		Direction: domain.SyncDirection(direction),
		Status:    domain.SyncStatus(status),
		Message:   message,
		Pushed:    pushed,
		Updated:   updated,
		Conflicts: conflicts,
		Timestamp: t,
	}, nil
}

// encryptData 使用 AES-GCM 加密数据。
// 复用 password 加密逻辑，支持任意字符串数据的加密。
func (s *Store) encryptData(plaintext string) ([]byte, error) {
	if plaintext == "" {
		return []byte{}, nil
	}

	key, err := s.getCryptoKey()
	if err != nil {
		return nil, err
	}
	if key == nil {
		if err := s.generateAndStoreKey(); err != nil {
			return nil, err
		}
		key, err = s.getCryptoKey()
		if err != nil {
			return nil, err
		}
	}

	return crypto.EncryptGCM(key, []byte(plaintext))
}

// decryptData 使用 AES-GCM 解密数据。
func (s *Store) decryptData(ciphertext []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", nil
	}

	key, err := s.getCryptoKey()
	if err != nil {
		return "", err
	}

	plaintext, err := crypto.DecryptGCM(key, ciphertext)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// CloudAccountData 表示 JSON 序列化的云账户数据（用于导出/导入）。
type CloudAccountData struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key,omitempty"`
	Bucket    string `json:"bucket"`
	Zone      string `json:"zone"`
	Enabled   bool   `json:"enabled"`
}

// MarshalAccountJSON 将云账户序列化为 JSON。
func (s *Store) MarshalAccountJSON(acct *domain.CloudAccount) ([]byte, error) {
	data := CloudAccountData{
		Name:      acct.Name,
		Type:      acct.Type,
		AccessKey: acct.AccessKey,
		Bucket:    acct.Bucket,
		Zone:      acct.Zone,
		Enabled:   acct.Enabled,
	}
	// 不序列化 SecretKey（用占位符代替）
	if acct.SecretKey != "" {
		data.SecretKey = "***"
	}
	return json.MarshalIndent(data, "", "  ")
}

// boolToInt 将 bool 转换为 int（SQLite 不支持 bool）。
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// intToBool 将 int 转换为 bool。
func intToBool(i int) bool {
	return i != 0
}

// GetLatestSyncTimestamp 获取最近一次同步的时间戳。
// 如果没有同步记录，返回零值 time.Time。
func (s *Store) GetLatestSyncTimestamp() (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ts string
	err := s.db.QueryRow("SELECT MAX(timestamp) FROM sync_history").Scan(&ts)
	if err != nil || ts == "" {
		return time.Time{}, nil
	}

	t, err := time.Parse("2006-01-02 15:04:05", ts)
	if err != nil {
		return time.Time{}, nil
	}
	return t, nil
}
