package port

import (
	"assh/asshc/domain"
	"golang.org/x/crypto/ssh"
)

// SSHConnector 定义 SSH 连接管理接口。
// 负责根据服务器配置建立 SSH 连接，以及关闭连接。
// 连接过程包括认证方式选择、HostKey 校验和 Keepalive 配置。
type SSHConnector interface {
	// Connect 根据服务器配置建立 SSH 连接，返回已认证的 ssh.Client。
	Connect(server *domain.Server) (*ssh.Client, error)
	// Close 关闭 SSH 连接并释放相关资源。
	Close(client *ssh.Client) error
}
