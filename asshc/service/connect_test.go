package service

import (
	"testing"

	"assh/asshc/domain"
	"golang.org/x/crypto/ssh"
)

type mockConnector struct {
	connectFunc     func(server *domain.Server) (*ssh.Client, error)
	connectChainFunc func(target *domain.Server, chain []*domain.Server) (interface{}, error)
}

func (m *mockConnector) Connect(server *domain.Server) (*ssh.Client, error) {
	if m.connectFunc != nil {
		return m.connectFunc(server)
	}
	return &ssh.Client{}, nil
}

func (m *mockConnector) Close(client *ssh.Client) error {
	return nil
}

func (m *mockConnector) ConnectChain(target *domain.Server, chain []*domain.Server) (interface{}, error) {
	if m.connectChainFunc != nil {
		return m.connectChainFunc(target, chain)
	}
	// 默认返回空的 ssh.Client
	return &ssh.Client{}, nil
}

type mockSession struct {
	shellFunc         func(client *ssh.Client) error
	runFunc           func(client *ssh.Client, cmd string) error
	runWithOutputFunc func(client *ssh.Client, cmd string) (string, error)
}

func (m *mockSession) Shell(client *ssh.Client) error {
	if m.shellFunc != nil {
		return m.shellFunc(client)
	}
	return nil
}

func (m *mockSession) Run(client *ssh.Client, cmd string) error {
	if m.runFunc != nil {
		return m.runFunc(client, cmd)
	}
	return nil
}

func (m *mockSession) RunWithOutput(client *ssh.Client, cmd string) (string, error) {
	if m.runWithOutputFunc != nil {
		return m.runWithOutputFunc(client, cmd)
	}
	// 默认返回命令输出（模拟执行成功）
	return "mock output for: " + cmd, nil
}

func TestNewConnectService(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})
	if svc == nil {
		t.Fatal("NewConnectService should not return nil")
	}
}

func TestConnectByName(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"prod": {"web-01": {Name: "web-01", Group: "prod", Host: "192.168.1.1", Port: 22, User: "root"}},
		},
	}
	svc := NewConnectService(&mockConnector{}, &mockSession{}, repo)

	client, err := svc.ConnectByName("web-01")
	if err != nil {
		t.Errorf("ConnectByName failed: %v", err)
	}
	if client == nil {
		t.Error("ConnectByName should return non-nil client")
	}
}

func TestConnectByNameNotFound(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	_, err := svc.ConnectByName("nonexistent")
	if err == nil {
		t.Error("ConnectByName should return error for nonexistent server")
	}
}

func TestConnectByNameEmptyName(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	_, err := svc.ConnectByName("")
	if err != domain.ErrInvalidName {
		t.Errorf("expected ErrInvalidName, got %v", err)
	}
}

func TestConnectDirect(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	client, err := svc.ConnectDirect("192.168.1.1", 22, "root", "secret", "", "")
	if err != nil {
		t.Errorf("ConnectDirect failed: %v", err)
	}
	if client == nil {
		t.Error("ConnectDirect should return non-nil client")
	}
}

func TestConnectDirectDefaults(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	client, err := svc.ConnectDirect("192.168.1.1", 0, "", "", "", "")
	if err != nil {
		t.Errorf("ConnectDirect with defaults failed: %v", err)
	}
	if client == nil {
		t.Error("ConnectDirect should return non-nil client")
	}
}

func TestConnectDirectEmptyHost(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	_, err := svc.ConnectDirect("", 22, "root", "", "", "")
	if err == nil {
		t.Error("ConnectDirect with empty host should return error")
	}
}

func TestShell(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	err := svc.Shell(&ssh.Client{})
	if err != nil {
		t.Errorf("Shell failed: %v", err)
	}
}

func TestShellNilClient(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	err := svc.Shell(nil)
	if err == nil {
		t.Error("Shell with nil client should return error")
	}
}

func TestRun(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	err := svc.Run(&ssh.Client{}, "ls")
	if err != nil {
		t.Errorf("Run failed: %v", err)
	}
}

func TestRunNilClient(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	err := svc.Run(nil, "ls")
	if err == nil {
		t.Error("Run with nil client should return error")
	}
}

func TestRunEmptyCommand(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	err := svc.Run(&ssh.Client{}, "")
	if err == nil {
		t.Error("Run with empty command should return error")
	}
}

func TestRunWithOutput(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	output, err := svc.RunWithOutput(&ssh.Client{}, "echo hello")
	if err != nil {
		t.Errorf("RunWithOutput failed: %v", err)
	}
	if output == "" {
		t.Error("RunWithOutput should return output")
	}
}

func TestRunWithOutputNilClient(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	_, err := svc.RunWithOutput(nil, "echo hello")
	if err == nil {
		t.Error("RunWithOutput with nil client should return error")
	}
}

func TestRunWithOutputEmptyCommand(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	_, err := svc.RunWithOutput(&ssh.Client{}, "")
	if err == nil {
		t.Error("RunWithOutput with empty command should return error")
	}
}

func TestClose(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	err := svc.Close(&ssh.Client{})
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestCloseNilClient(t *testing.T) {
	svc := NewConnectService(&mockConnector{}, &mockSession{}, &mockRepo{})

	err := svc.Close(nil)
	if err != nil {
		t.Errorf("Close with nil client should not return error: %v", err)
	}
}
