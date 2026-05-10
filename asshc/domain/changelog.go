package domain

// ChangelogEntry 记录服务器配置的一次变更历史。
// 每次配置变更都会在变更日志中创建一个条目，包含变更前后的全量快照。
type ChangelogEntry struct {
	ID         int     `json:"id"`          // 日志条目 ID
	ServerName string  `json:"server_name"` // 服务器名称
	GroupName  string  `json:"group_name"`  // 服务器分组
	Version    int     `json:"version"`     // 变更后的版本号
	ChangeType string  `json:"change_type"` // 变更类型：create / update / rollback
	Snapshot   *Server `json:"snapshot"`    // 该版本的全量服务器配置快照
	CreatedAt  string  `json:"created_at"`  // 变更时间
}

// 变更类型常量，标识服务器配置的变更操作类别。
const (
	ChangeTypeCreate   = "create"   // 新建服务器
	ChangeTypeUpdate   = "update"   // 更新配置
	ChangeTypeRollback = "rollback" // 回滚到历史版本
)
