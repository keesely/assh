package domain

// ChangelogEntry records a server configuration change.
type ChangelogEntry struct {
	ID         int     `json:"id"`
	ServerName string  `json:"server_name"`
	GroupName  string  `json:"group_name"`
	Version    int     `json:"version"`
	ChangeType string  `json:"change_type"` // "create", "update", "rollback"
	Snapshot   *Server `json:"snapshot"`    // full server config at this version
	CreatedAt  string  `json:"created_at"`
}

const (
	ChangeTypeCreate   = "create"
	ChangeTypeUpdate   = "update"
	ChangeTypeRollback = "rollback"
)
