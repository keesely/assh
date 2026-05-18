package port

import "assh/asshc/domain"

// JumpHistoryRecorder 定义跳板历史记录存储接口。
// 负责跳板连接历史记录的持久化、查询和管理。
type JumpHistoryRecorder interface {
	// Record 记录一次跳板链使用。
	Record(jh *domain.JumpHistory) error

	// List 返回历史记录，按 last_used 降序排列。
	List(limit int) ([]*domain.JumpHistory, error)

	// Get 根据 ID 获取历史记录。
	Get(id int64) (*domain.JumpHistory, error)

	// Delete 删除指定历史记录。
	Delete(id int64) error

	// Clear 清空所有历史记录。
	Clear() error

	// IncrementUse 递增使用次数并更新 last_used。
	IncrementUse(id int64) error
}