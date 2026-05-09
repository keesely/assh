package port

import (
	"net"
	"golang.org/x/crypto/ssh"
)

type HostKeyCallback func(hostname string, remote net.Addr, key ssh.PublicKey) error

type HostKeyChecker interface {
	Check(hostname string, remote net.Addr, key ssh.PublicKey) error
}
