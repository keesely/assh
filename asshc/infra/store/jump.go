package store

import (
	"database/sql"
	"encoding/json"

	"assh/asshc/domain"
	"assh/asshc/port"
	"assh/log"
)

// JumpRecorder 实现 port.JumpHistoryRecorder 接口。
// 使用 Store 的 cryptoKey 进行 path_data 的 AES-GCM 加密。
type JumpRecorder struct {
	store *Store
}

// NewJumpRecorder 创建 JumpRecorder 实例。
func NewJumpRecorder(store *Store) *JumpRecorder {
	return &JumpRecorder{store: store}
}

// Record 记录一次跳板链使用。
func (r *JumpRecorder) Record(jh *domain.JumpHistory) error {
	// 序列化 PathData 为 JSON
	pathDataJSON, err := json.Marshal(jh.PathData)
	if err != nil {
		return err
	}

	// 加密 path_data
	var encryptedPathData []byte
	if len(pathDataJSON) > 0 {
		encryptedPathData, err = r.store.encryptData(string(pathDataJSON))
		if err != nil {
			return err
		}
	}

	_, err = r.store.db.Exec(`
		INSERT INTO jump_history (target_expr, path_text, path_data, chain_count, last_used, use_count)
		VALUES (?, ?, ?, ?, datetime('now'), 1)
	`, jh.TargetExpr, jh.PathText, encryptedPathData, jh.ChainCount)

	return err
}

// List 返回历史记录，按 last_used 降序排列。
func (r *JumpRecorder) List(limit int) ([]*domain.JumpHistory, error) {
	rows, err := r.store.db.Query(`
		SELECT id, target_expr, path_text, path_data, chain_count, last_used, use_count, created_at, updated_at
		FROM jump_history
		ORDER BY last_used DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*domain.JumpHistory
	for rows.Next() {
		jh := &domain.JumpHistory{}
		var pathDataEncrypted sql.RawBytes
		err := rows.Scan(&jh.ID, &jh.TargetExpr, &jh.PathText, &pathDataEncrypted, &jh.ChainCount, &jh.LastUsed, &jh.UseCount, &jh.CreatedAt, &jh.UpdatedAt)
		if err != nil {
			return nil, err
		}
		// 解密 path_data
		if len(pathDataEncrypted) > 0 {
			decrypted, err := r.store.decryptData(pathDataEncrypted)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal([]byte(decrypted), &jh.PathData); err != nil {
				log.Warnf("failed to parse jump history path data: %v", err)
			}
		}
		results = append(results, jh)
	}
	return results, nil
}

// Get 根据 ID 获取历史记录。
func (r *JumpRecorder) Get(id int64) (*domain.JumpHistory, error) {
	row := r.store.db.QueryRow(`
		SELECT id, target_expr, path_text, path_data, chain_count, last_used, use_count, created_at, updated_at
		FROM jump_history
		WHERE id = ?
	`, id)

	jh := &domain.JumpHistory{}
	var pathDataEncrypted sql.RawBytes
	err := row.Scan(&jh.ID, &jh.TargetExpr, &jh.PathText, &pathDataEncrypted, &jh.ChainCount, &jh.LastUsed, &jh.UseCount, &jh.CreatedAt, &jh.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// 解密 path_data
	if len(pathDataEncrypted) > 0 {
		decrypted, err := r.store.decryptData(pathDataEncrypted)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(decrypted), &jh.PathData); err != nil {
			log.Warnf("failed to parse jump history path data: %v", err)
		}
	}

	return jh, nil
}

// Delete 删除指定历史记录。
func (r *JumpRecorder) Delete(id int64) error {
	_, err := r.store.db.Exec("DELETE FROM jump_history WHERE id = ?", id)
	return err
}

// Clear 清空所有历史记录。
func (r *JumpRecorder) Clear() error {
	_, err := r.store.db.Exec("DELETE FROM jump_history")
	return err
}

// IncrementUse 递增使用次数并更新 last_used。
func (r *JumpRecorder) IncrementUse(id int64) error {
	_, err := r.store.db.Exec(`
		UPDATE jump_history
		SET use_count = use_count + 1, last_used = datetime('now')
		WHERE id = ?
	`, id)
	return err
}

// Ensure JumpRecorder implements port.JumpHistoryRecorder
var _ port.JumpHistoryRecorder = (*JumpRecorder)(nil)