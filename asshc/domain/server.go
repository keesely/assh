// Package domain 定义 ASSH 应用的核心领域模型。
//
// 本包包含最基础的业务实体（Server、Auth）和通用错误定义，
// 不依赖任何外部基础设施。其他层（port、service、infra）均
// 引用本包定义的类型。
package domain

import (
	"fmt"
	"strings"
)

// Auth 表示服务器的认证信息，包含密码和密钥文件两种方式。
// 两种方式可以同时存在，连接时按顺序尝试。
type Auth struct {
	Password string `json:"password,omitempty"` // 密码认证
	KeyFile  string `json:"keyfile,omitempty"`  // 密钥文件路径
}

// Server 表示一台 SSH 服务器的完整配置信息。
// Name 和 Group 共同构成唯一标识，格式为 "group.name"。
// Auth 字段可空，Version 由持久化层自动维护。
type Server struct {
	Name    string                 `json:"name"`             // 服务器名称（组内唯一）
	Group   string                 `json:"group"`            // 服务器分组名
	Host    string                 `json:"host"`             // 主机地址（IP 或域名）
	Port    int                    `json:"port"`             // SSH 端口号（默认 22）
	User    string                 `json:"user"`             // 登录用户名（默认 root）
	Auth    *Auth                  `json:"auth,omitempty"`   // 认证信息（可选）
	Remark  string                 `json:"remark,omitempty"` // 备注信息
	Options map[string]interface{} `json:"options,omitempty"` // 扩展选项（如 keepalive）
	Version int                    `json:"version"`          // 配置版本号，每次更新递增
}

// ParseName 解析复合名称 "group.name" 为分组和服务器名两部分。
// 如果名称中不包含 "."，则分组为空字符串。
func ParseName(name string) (group, serverName string) {
	if name == "" {
		return "", ""
	}

	parts := strings.SplitN(name, ".", 2)
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}

// JoinName 将分组和服务器名合并为 "group.name" 格式。
// 如果分组为空，只返回服务器名。
func JoinName(group, name string) string {
	if group == "" {
		return name
	}
	return fmt.Sprintf("%s.%s", group, name)
}

// ValidateServer 校验服务器配置的必填字段和取值范围。
// 检查 Name 非空、Host 非空、Port 在 1-65535 范围内。
func ValidateServer(s *Server) error {
	if s.Name == "" {
		return ErrInvalidName
	}
	if s.Host == "" {
		return ErrEmptyField
	}
	if s.Port < 1 || s.Port > 65535 {
		return ErrInvalidPort
	}
	return nil
}

// NewServer 创建一个新的 Server 实例，设置默认值。
// port 为 0 时默认使用 22，user 为空时默认使用 "root"。
// 自动从 name 中解析出 group 信息。
func NewServer(name, host string, port int, user string) *Server {
	if port == 0 {
		port = 22
	}
	if user == "" {
		user = "root"
	}

	s := &Server{
		Name:    name,
		Host:    host,
		Port:    port,
		User:    user,
		Options: make(map[string]interface{}),
	}

	group, _ := ParseName(name)
	s.Group = group

	return s
}