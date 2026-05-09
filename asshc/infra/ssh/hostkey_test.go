package ssh

import (
	"testing"
)

func TestNewKnownHostsChecker(t *testing.T) {
	k := NewKnownHostsChecker("~/.ssh/known_hosts")
	if k == nil {
		t.Error("NewKnownHostsChecker should not return nil")
	}
	if k.knownHostsPath != "~/.ssh/known_hosts" {
		t.Errorf("unexpected path: %s", k.knownHostsPath)
	}
}

func TestNewInsecureChecker(t *testing.T) {
	i := NewInsecureChecker()
	if i == nil {
		t.Error("NewInsecureChecker should not return nil")
	}
}
