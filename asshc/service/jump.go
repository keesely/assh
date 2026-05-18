package service

import (
	"assh/asshc/domain"
	"assh/asshc/port"
)

// JumpService 跳板历史服务，负责跳板链的记录和历史管理。
type JumpService struct {
	recorder port.JumpHistoryRecorder
}

// NewJumpService 创建 JumpService 实例。
func NewJumpService(recorder port.JumpHistoryRecorder) *JumpService {
	return &JumpService{recorder: recorder}
}

// RecordJumpChain 记录一次跳板链使用。
// targetExpr: 目标表达式（服务器名或 "user@host:port"）
// pathText: 人类可读的跳板路径文本
// hops: 跳板链中的服务器列表
func (s *JumpService) RecordJumpChain(targetExpr, pathText string, hops []*domain.Server) error {
	// 转换 hops 为 JumpHopAuth 列表
	pathData := make([]domain.JumpHopAuth, len(hops))
	for i, hop := range hops {
		pathData[i] = domain.JumpHopAuth{
			Type:      "server",
			ServerRef: hop.Name,
			Host:      hop.Host,
			Port:      hop.Port,
			User:      hop.User,
		}
	}

	jh := &domain.JumpHistory{
		TargetExpr: targetExpr,
		PathText:   pathText,
		PathData:   pathData,
		ChainCount: len(hops),
	}

	return s.recorder.Record(jh)
}

// List 列出历史记录。
func (s *JumpService) List(limit int) ([]*domain.JumpHistory, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.recorder.List(limit)
}

// Delete 删除历史记录。
func (s *JumpService) Delete(id int64) error {
	return s.recorder.Delete(id)
}

// Clear 清空历史记录。
func (s *JumpService) Clear() error {
	return s.recorder.Clear()
}