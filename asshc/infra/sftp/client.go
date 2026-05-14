package sftp

import (
	"fmt"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	sftpConcurrentReads  = true
	sftpConcurrentWrites = true
	sftpMaxRequests      = 64
)

type SFTPSession struct {
	client    *sftp.Client
	sshClient *ssh.Client
}

func NewSFTPSession(sshClient *ssh.Client) (*SFTPSession, error) {
	client, err := sftp.NewClient(sshClient,
		sftp.UseConcurrentReads(sftpConcurrentReads),
		sftp.UseConcurrentWrites(sftpConcurrentWrites),
		sftp.MaxConcurrentRequestsPerFile(sftpMaxRequests),
	)
	if err != nil {
		return nil, fmt.Errorf("sftp client creation failed: %w", err)
	}

	return &SFTPSession{
		client:    client,
		sshClient: sshClient,
	}, nil
}

func (s *SFTPSession) Client() *sftp.Client {
	return s.client
}

func (s *SFTPSession) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func (s *SFTPSession) IsClosed() bool {
	return s.client == nil
}

func (s *SFTPSession) Reconnect() error {
	if s.sshClient == nil {
		return fmt.Errorf("ssh client is not available")
	}

	newClient, err := sftp.NewClient(s.sshClient,
		sftp.UseConcurrentReads(sftpConcurrentReads),
		sftp.UseConcurrentWrites(sftpConcurrentWrites),
		sftp.MaxConcurrentRequestsPerFile(sftpMaxRequests),
	)
	if err != nil {
		return fmt.Errorf("sftp reconnect failed: %w", err)
	}

	if s.client != nil {
		s.client.Close()
	}
	s.client = newClient
	return nil
}