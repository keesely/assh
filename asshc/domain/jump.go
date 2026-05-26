package domain

// JumpHopAuth 记录一跳的认证信息。
type JumpHopAuth struct {
	Type      string `json:"type"`                 // 认证类型：server / direct
	ServerRef string `json:"server_ref,omitempty"` // 服务器名引用（type=server）
	Host      string `json:"host,omitempty"`       // 直接主机地址（type=direct）
	Port      int    `json:"port,omitempty"`        // 端口
	User      string `json:"user,omitempty"`        // 用户名
	Password  string `json:"password,omitempty"`    // 明文密码（持久化时加密）
	KeyFile   string `json:"keyfile,omitempty"`     // 密钥文件路径
}

// JumpHistory 表示一次跳板连接的历史记录。
// 记录用户通过跳板链连接到目标服务器的历史，包括路径信息、使用次数等。
type JumpHistory struct {
	// ID 历史记录唯一标识
	ID int64 `json:"id"`

	// TargetExpr 目标表达式，服务器名或 "user@host:port" 格式
	TargetExpr string `json:"target_expr"`

	// PathText 人类可读的跳板路径文本，如 "bastion,gateway,root@10.0.1.1"
	PathText string `json:"path_text"`

	// PathData 加密持久化的每跳认证信息（不在 JSON 中暴露）
	PathData []JumpHopAuth `json:"-"`

	// ChainCount 跳板链中的跳数
	ChainCount int `json:"chain_count"`

	// LastUsed 最后使用时间
	LastUsed string `json:"last_used"`

	// UseCount 累计使用次数
	UseCount int `json:"use_count"`

	// CreatedAt 创建时间
	CreatedAt string `json:"created_at"`

	// UpdatedAt 更新时间
	UpdatedAt string `json:"updated_at"`
}