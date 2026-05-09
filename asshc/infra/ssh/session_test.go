package ssh

import (
	"testing"
)

func TestNewSession(t *testing.T) {
	s := NewSession()
	if s == nil {
		t.Fatal("NewSession should not return nil")
	}
}
