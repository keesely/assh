package ssh

import (
	"net"

	"golang.org/x/crypto/ssh"
)

type KnownHostsChecker struct {
	knownHostsPath string
}

func NewKnownHostsChecker(path string) *KnownHostsChecker {
	return &KnownHostsChecker{knownHostsPath: path}
}

func (k *KnownHostsChecker) Check(hostname string, remote net.Addr, key ssh.PublicKey) error {
	return nil
}

type InsecureChecker struct{}

func NewInsecureChecker() *InsecureChecker {
	return &InsecureChecker{}
}

func (i *InsecureChecker) Check(hostname string, remote net.Addr, key ssh.PublicKey) error {
	return nil
}
