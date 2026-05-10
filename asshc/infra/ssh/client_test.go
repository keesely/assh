package ssh

import (
	"testing"

	"assh/asshc/domain"
)

func TestNewConnector(t *testing.T) {
	c := NewConnector()
	if c == nil {
		t.Fatal("NewConnector should not return nil")
	}
}

func TestAuthMethodsPriority(t *testing.T) {
	c := &Connector{}
	server := &domain.Server{
		Host: "192.168.1.1",
		Port: 22,
		User: "root",
		Auth: &domain.Auth{
			Password: "testpass",
			KeyFile:  "/tmp/nonexistent_key",
		},
	}

	methods := c.authMethods(server)
	if len(methods) == 0 {
		t.Error("should have at least password auth method")
	}
}

func TestAuthMethodsPasswordOnly(t *testing.T) {
	c := &Connector{}
	server := &domain.Server{
		Host: "192.168.1.1",
		Port: 22,
		User: "root",
		Auth: &domain.Auth{
			Password: "secret",
		},
	}

	methods := c.authMethods(server)
	if len(methods) == 0 {
		t.Error("should have password auth method when password is set")
	}
}

func TestAuthMethodsEmpty(t *testing.T) {
	c := &Connector{}
	server := &domain.Server{
		Host: "192.168.1.1",
		Port: 22,
		User: "root",
	}

	methods := c.authMethods(server)
	_ = methods
}

func TestGetHostKeyCallback(t *testing.T) {
	c := &Connector{}

	server1 := &domain.Server{Host: "test", Options: map[string]interface{}{}}
	cb1 := c.getHostKeyCallback(server1)
	if cb1 == nil {
		t.Error("getHostKeyCallback should not return nil")
	}

	server2 := &domain.Server{Host: "test", Options: map[string]interface{}{
		"insecure_skip_hostkey": true,
	}}
	cb2 := c.getHostKeyCallback(server2)
	if cb2 == nil {
		t.Error("getHostKeyCallback should not return nil when skip=true")
	}

	server3 := &domain.Server{Host: "test", Options: map[string]interface{}{
		"insecure_skip_hostkey": false,
	}}
	cb3 := c.getHostKeyCallback(server3)
	if cb3 == nil {
		t.Error("getHostKeyCallback should not return nil when skip=false")
	}
}

func TestGetKeepalive(t *testing.T) {
	c := &Connector{}

	server1 := &domain.Server{Options: map[string]interface{}{}}
	if v := c.getKeepalive(server1); v != 0 {
		t.Errorf("expected 0, got %d", v)
	}

	server2 := &domain.Server{Options: map[string]interface{}{
		"keepalive": float64(30),
	}}
	if v := c.getKeepalive(server2); v != 30 {
		t.Errorf("expected 30, got %d", v)
	}
}

func TestCloseNilClient(t *testing.T) {
	c := &Connector{}
	err := c.Close(nil)
	if err != nil {
		t.Errorf("Close(nil) should not error: %v", err)
	}
}
