package domain

import "time"

// ProxyService 表示代理服务配置。
//
// 用于管理 SOCKS5、HTTP、TCP 和 Smart 代理服务的配置信息。
// Auth 字段存储明文，持久化时使用 AES-GCM 加密。
type ProxyService struct {
	// ID 数据库主键
	ID int64 `json:"id"`

	// Name 服务名称
	Name string `json:"name"`

	// GroupName 分组名称
	GroupName string `json:"group_name"`

	// ServerName 关联的服务器名称
	ServerName string `json:"server_name"`

	// Host 主机地址
	Host string `json:"host"`

	// Port 端口号
	Port int `json:"port"`

	// UserName 用户名
	UserName string `json:"user_name"`

	// JumpChain 跳板链配置
	JumpChain string `json:"jump_chain"`

	// ProxyType 代理类型：socks5|http|tcp|smart
	ProxyType string `json:"proxy_type"`

	// SOCKS5Addr SOCKS5 监听地址
	SOCKS5Addr string `json:"socks5_addr"`

	// HTTPAddr HTTP 代理监听地址
	HTTPAddr string `json:"http_addr"`

	// TCPAddr TCP 转发地址
	TCPAddr string `json:"tcp_addr"`

	// Reverse 是否反向代理
	Reverse bool `json:"reverse"`

	// ListenType 监听类型：tcp|http
	ListenType string `json:"listen_type"`

	// Auth 认证信息，格式 "user:pass"（明文，存储时加密）
	Auth string `json:"-"`

	// RuleFile 规则文件路径
	RuleFile string `json:"rule_file"`

	// LogDir 日志目录
	LogDir string `json:"log_dir"`

	// Daemon 是否以守护进程运行
	Daemon bool `json:"daemon"`

	// AutoReconnect 自动重连配置
	AutoReconnect string `json:"auto_reconnect"`

	// Remark 备注
	Remark string `json:"remark"`

	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// UseCount 累计使用次数
	UseCount int `json:"use_count"`

	// LastUsed 最后使用时间
	LastUsed *time.Time `json:"last_used"`

	// Version 配置版本号
	Version int `json:"version"`

	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}