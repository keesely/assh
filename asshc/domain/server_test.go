package domain

import (
	"testing"
)

func TestParseName(t *testing.T) {
	tests := []struct {
		input    string
		wantGroup   string
		wantName    string
	}{
		{"web-01", "", "web-01"},
		{"prod.web-01", "prod", "web-01"},
		{"dev.staging.db-01", "dev", "staging.db-01"},
		{"", "", ""},
	}

	for _, tt := range tests {
		gotGroup, gotName := ParseName(tt.input)
		if gotGroup != tt.wantGroup || gotName != tt.wantName {
			t.Errorf("ParseName(%q) = (%q, %q), want (%q, %q)",
				tt.input, gotGroup, gotName, tt.wantGroup, tt.wantName)
		}
	}
}

func TestJoinName(t *testing.T) {
	tests := []struct {
		group  string
		name   string
		want   string
	}{
		{"prod", "web-01", "prod.web-01"},
		{"", "web-01", "web-01"},
		{"dev.staging", "db-01", "dev.staging.db-01"},
	}

	for _, tt := range tests {
		got := JoinName(tt.group, tt.name)
		if got != tt.want {
			t.Errorf("JoinName(%q, %q) = %q, want %q", tt.group, tt.name, got, tt.want)
		}
	}
}

func TestValidateServer(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		port   int
		wantErr bool
	}{
		{"", "", 22, true},
		{"valid", "", 22, true},
		{"valid", "192.168.1.1", 0, true},
		{"valid", "192.168.1.1", -1, true},
		{"valid", "192.168.1.1", 65536, true},
		{"valid", "192.168.1.1", 22, false},
		{"valid", "example.com", 8080, false},
	}

	for _, tt := range tests {
		server := &Server{
			Name:  tt.name,
			Host:  tt.host,
			Port:  tt.port,
			User:  "root",
		}
		err := ValidateServer(server)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateServer(name=%q, host=%q, port=%d) error = %v, wantErr = %v",
				tt.name, tt.host, tt.port, err, tt.wantErr)
		}
	}
}

func TestNewServer(t *testing.T) {
	server := NewServer("test-server", "192.168.1.1", 22, "root")

	if server.Name != "test-server" {
		t.Errorf("Name = %q, want %q", server.Name, "test-server")
	}
	if server.Host != "192.168.1.1" {
		t.Errorf("Host = %q, want %q", server.Host, "192.168.1.1")
	}
	if server.Port != 22 {
		t.Errorf("Port = %d, want %d", server.Port, 22)
	}
	if server.User != "root" {
		t.Errorf("User = %q, want %q", server.User, "root")
	}
}

func TestAuthStructure(t *testing.T) {
	auth := &Auth{
		Password: "secret",
		KeyFile:  "~/.ssh/id_rsa",
	}

	if auth.Password != "secret" {
		t.Errorf("Password = %q, want %q", auth.Password, "secret")
	}
	if auth.KeyFile != "~/.ssh/id_rsa" {
		t.Errorf("KeyFile = %q, want %q", auth.KeyFile, "~/.ssh/id_rsa")
	}
}

func TestServerStructure(t *testing.T) {
	server := &Server{
		Name:    "web-01",
		Group:   "prod",
		Host:    "192.168.1.1",
		Port:    22,
		User:    "admin",
		Auth:    &Auth{Password: "secret"},
		Remark:  "Production web server",
		Options: map[string]interface{}{"keepalive": 30},
	}

	if server.Name != "web-01" {
		t.Errorf("Name = %q, want %q", server.Name, "web-01")
	}
	if server.Group != "prod" {
		t.Errorf("Group = %q, want %q", server.Group, "prod")
	}
	if server.Auth == nil || server.Auth.Password != "secret" {
		t.Error("Auth should have Password 'secret'")
	}
	if server.Options["keepalive"] != 30 {
		t.Errorf("Options[keepalive] = %v, want 30", server.Options["keepalive"])
	}
}