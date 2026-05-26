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
//
// 跳板类型识别规则：
//   - hop.Name != "" → type "server"（从 DB 读取的已保存服务器），ServerRef = hop.Name
//   - hop.Name == "" → type "direct"（user@host[:port] 格式直接指定），无 ServerRef
//
// 认证信息（Password/KeyFile）从 hop.Auth 中记录，用于 -J @id 恢复时复用。
func (s *JumpService) RecordJumpChain(targetExpr, pathText string, hops []*domain.Server) error {
	// 转换 hops 为 JumpHopAuth 列表
	pathData := make([]domain.JumpHopAuth, len(hops))
	for i, hop := range hops {
		auth := domain.JumpHopAuth{
			Host: hop.Host,
			Port: hop.Port,
			User: hop.User,
		}

		if hop.Name != "" {
			// 已保存的服务器引用
			auth.Type = "server"
			auth.ServerRef = hop.Name
		} else {
			// user@host[:port] 直接指定
			auth.Type = "direct"
		}

		// 记录认证信息（用于 -J @id 恢复时复用）
		if hop.Auth != nil {
			auth.Password = hop.Auth.Password
			auth.KeyFile = hop.Auth.KeyFile
		}

		pathData[i] = auth
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