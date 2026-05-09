package port

import (
	"assh/asshc/domain"
	"golang.org/x/crypto/ssh"
)

type SSHConnector interface {
	Connect(server *domain.Server) (*ssh.Client, error)
	Close(client *ssh.Client) error
}
