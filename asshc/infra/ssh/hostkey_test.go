package ssh

import (
	"testing"
)

func TestNewKnownHostsChecker(t *testing.T) {
	k, err := NewKnownHostsChecker("~/.ssh/known_hosts")
	if err != nil {
		t.Fatalf("NewKnownHostsChecker failed: %v", err)
	}
	if k == nil {
		t.Error("NewKnownHostsChecker should not return nil")
	}
	if k.knownHostsPath == "" {
		t.Error("knownHostsPath should not be empty")
	}
}

func TestNewInsecureChecker(t *testing.T) {
	i := NewInsecureChecker()
	if i == nil {
		t.Error("NewInsecureChecker should not return nil")
	}
}
