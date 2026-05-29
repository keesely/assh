package domain

import "time"

// TunnelForward 端口转发规则
type TunnelForward struct {
	LocalAddr  string `json:"local"`
	RemoteAddr string `json:"remote"`
	Reverse    bool   `json:"reverse"`
}

// TunnelService 隧道服务配置
type TunnelService struct {
	ID            int64
	Name          string
	GroupName     string
	ServerName    string
	Host          string
	Port          int
	UserName      string
	JumpChain     string
	Forwards      []TunnelForward
	Auth          string // user:pass (明文，存储时加密)
	Daemon        bool
	AutoReconnect string
	Remark        string
	Enabled       bool
	UseCount      int
	LastUsed      *time.Time
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
