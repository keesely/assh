package port

import "golang.org/x/crypto/ssh"

type SSHSession interface {
	Shell(client *ssh.Client) error
	Run(client *ssh.Client, cmd string) error
	RunWithOutput(client *ssh.Client, cmd string) (string, error)
}
