package service

import (
	"fmt"

	"assh/asshc/domain"
	"assh/asshc/port"
	"golang.org/x/crypto/ssh"
)

type ConnectService struct {
	connector port.SSHConnector
	session   port.SSHSession
	repo      port.ServerRepository
}

func NewConnectService(
	connector port.SSHConnector,
	session port.SSHSession,
	repo port.ServerRepository,
) *ConnectService {
	return &ConnectService{
		connector: connector,
		session:   session,
		repo:      repo,
	}
}

func (s *ConnectService) ConnectByName(name string) (*ssh.Client, error) {
	if name == "" {
		return nil, domain.ErrInvalidName
	}

	server, err := s.repo.Get(name)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, domain.ErrNotFound
	}

	return s.connector.Connect(server)
}

func (s *ConnectService) ConnectDirect(host string, port int, user, password, keyFile string) (*ssh.Client, error) {
	if host == "" {
		return nil, fmt.Errorf("host is required")
	}

	server := &domain.Server{
		Host: host,
		Port: port,
		User: user,
		Auth: &domain.Auth{
			Password: password,
			KeyFile:  keyFile,
		},
	}

	if server.Port <= 0 {
		server.Port = 22
	}
	if server.User == "" {
		server.User = "root"
	}

	return s.connector.Connect(server)
}

func (s *ConnectService) Shell(client *ssh.Client) error {
	if client == nil {
		return fmt.Errorf("ssh client is nil")
	}
	return s.session.Shell(client)
}

func (s *ConnectService) Run(client *ssh.Client, cmd string) error {
	if client == nil {
		return fmt.Errorf("ssh client is nil")
	}
	if cmd == "" {
		return fmt.Errorf("command is empty")
	}
	return s.session.Run(client, cmd)
}

func (s *ConnectService) RunWithOutput(client *ssh.Client, cmd string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("ssh client is nil")
	}
	if cmd == "" {
		return "", fmt.Errorf("command is empty")
	}
	return s.session.RunWithOutput(client, cmd)
}

func (s *ConnectService) Close(client *ssh.Client) error {
	if client == nil {
		return nil
	}
	return s.connector.Close(client)
}
