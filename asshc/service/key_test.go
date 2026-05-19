package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"assh/asshc/domain"
	"golang.org/x/crypto/ssh"
)

// mockKeyManager 实现 port.KeyManager 接口，用于 KeyService 单元测试。
// 每个方法使用可配置的函数字段，默认返回成功值。
type mockKeyManager struct {
	generateFunc       func(keyType string, bits int, passphrase []byte, comment string) (string, []byte, string, error)
	generateToPathFunc func(keyType string, bits int, passphrase []byte, comment string, outputPath string) ([]byte, string, error)
	deployFunc         func(server *domain.Server, pubKey []byte) error
	backupFunc         func(srcPath, destPath string) (string, error)
	backupPathFunc     func(name string) (string, bool)
	readStdinFunc      func() (string, string, error)
	getPassFunc        func() []byte
}

func (m *mockKeyManager) Generate(keyType string, bits int, passphrase []byte, comment string) (string, []byte, string, error) {
	if m.generateFunc != nil {
		return m.generateFunc(keyType, bits, passphrase, comment)
	}
	return "/tmp/keys/abc123.pem", []byte("ssh-rsa AAA..."), "abc123", nil
}

func (m *mockKeyManager) DeployPublicKey(server *domain.Server, pubKey []byte) error {
	if m.deployFunc != nil {
		return m.deployFunc(server, pubKey)
	}
	return nil
}

func (m *mockKeyManager) BackupKey(srcPath, destPath string) (string, error) {
	if m.backupFunc != nil {
		return m.backupFunc(srcPath, destPath)
	}
	return "def456", nil
}

func (m *mockKeyManager) BackupPath(name string) (string, bool) {
	if m.backupPathFunc != nil {
		return m.backupPathFunc(name)
	}
	return "/tmp/keys/" + sha256Name(name) + ".pem", false
}

func (m *mockKeyManager) ReadKeyFromStdin() (string, string, error) {
	if m.readStdinFunc != nil {
		return m.readStdinFunc()
	}
	return "/tmp/keys/stdin.pem", "stdin123", nil
}

func (m *mockKeyManager) GenerateToPath(keyType string, bits int, passphrase []byte, comment string, outputPath string) ([]byte, string, error) {
	if m.generateToPathFunc != nil {
		return m.generateToPathFunc(keyType, bits, passphrase, comment, outputPath)
	}
	return []byte("ssh-rsa AAA..."), "genpath123", nil
}

func (m *mockKeyManager) GenerateToExistingPath(keyType string, bits int, passphrase []byte, comment string, privPath string) ([]byte, error) {
	// 默认实现：返回公钥和成功
	return []byte("ssh-rsa AAA..."), nil
}

func (m *mockKeyManager) GetAccountPassphrase() []byte {
	if m.getPassFunc != nil {
		return m.getPassFunc()
	}
	return nil
}

// sha256Name 模拟 BackupPath 的哈希命名（用于测试断言）。
func sha256Name(name string) string {
	// 简单模拟：使用 name 的 hex 表示作为哈希
	return name // 测试中不依赖实际哈希值
}

// TestNewKeyService 验证 KeyService 构造函数正常创建实例。
func TestNewKeyService(t *testing.T) {
	svc := NewKeyService(&mockKeyManager{}, &mockRepo{}, &mockConnector{})
	if svc == nil {
		t.Fatal("NewKeyService should not return nil")
	}
}

// ---------------------------------------------------------------------------
// GenerateAndDeploy 测试
// ---------------------------------------------------------------------------

// TestGenerateAndDeploy_Success 验证成功场景：
// 生成密钥 → 部署公钥 → 更新配置 → 全部成功
// 注意：DeployService 需要真实 SSH 连接，这里模拟连接失败但主流程继续
func TestGenerateAndDeploy_Success(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"test": {Name: "test", Host: "10.0.0.1", User: "root"}},
		},
	}
	keymgr := &mockKeyManager{}
	// 提供一个返回错误的 connectFunc，使 DeployService 优雅失败（不中断主流程）
	connector := &mockConnectorWithDeployFail{}
	svc := NewKeyService(keymgr, repo, connector)

	err := svc.GenerateAndDeploy("test", "rsa", 4096, "")
	if err != nil {
		t.Fatalf("GenerateAndDeploy failed: %v", err)
	}

	// 验证配置已更新
	server, err := repo.Get("test")
	if err != nil {
		t.Fatalf("Get server failed: %v", err)
	}
	if server.Auth == nil || server.Auth.KeyFile == "" {
		t.Error("Auth.KeyFile should be set after GenerateAndDeploy")
	}
}

// mockConnectorWithDeployFail 是一个让 DeployService 失败的 mockConnector
type mockConnectorWithDeployFail struct{}

func (c *mockConnectorWithDeployFail) Connect(server *domain.Server) (*ssh.Client, error) {
	return nil, errors.New("mock SSH connection error for deploy test")
}

func (c *mockConnectorWithDeployFail) Close(client *ssh.Client) error {
	return nil
}

func (c *mockConnectorWithDeployFail) ConnectChain(target *domain.Server, chain []*domain.Server) (*ssh.Client, error) {
	return nil, errors.New("mock SSH connection error for deploy test")
}

// TestGenerateAndDeploy_EmptyName 验证空服务器名返回 ErrInvalidName。
func TestGenerateAndDeploy_EmptyName(t *testing.T) {
	svc := NewKeyService(&mockKeyManager{}, &mockRepo{}, &mockConnector{})

	err := svc.GenerateAndDeploy("", "rsa", 4096, "")
	if err != domain.ErrInvalidName {
		t.Errorf("expected ErrInvalidName, got %v", err)
	}
}

// TestGenerateAndDeploy_ServerNotFound 验证不存在的服务器返回错误。
func TestGenerateAndDeploy_ServerNotFound(t *testing.T) {
	svc := NewKeyService(&mockKeyManager{}, &mockRepo{}, &mockConnector{})

	err := svc.GenerateAndDeploy("nonexist", "rsa", 4096, "")
	if err == nil {
		t.Fatal("expected error for nonexistent server")
	}
}

// TestGenerateAndDeploy_GenerateFails 验证密钥生成失败时返回错误。
func TestGenerateAndDeploy_GenerateFails(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"test": {Name: "test", Host: "10.0.0.1"}},
		},
	}
	keymgr := &mockKeyManager{
		generateFunc: func(keyType string, bits int, passphrase []byte, comment string) (string, []byte, string, error) {
			return "", nil, "", errors.New("generate failed")
		},
	}
		svc := NewKeyService(keymgr, repo, &mockConnectorWithDeployFail{})

	err := svc.GenerateAndDeploy("test", "rsa", 4096, "")
	if err == nil {
		t.Fatal("expected error when generate fails")
	}
}

// TestGenerateAndDeploy_DeployStub 验证部署成功时 KeyService.GenerateAndDeploy 正常完成。
// 注意：DeployStubError 不再使用，部署通过 DeployService 执行。
// 由于 DeployService 需要真实 SSH 连接，这里使用 mockConnectorWithDeployFail 模拟部署失败，
// 但主流程仍然成功（部署失败不影响主流程）。
func TestGenerateAndDeploy_DeployStub(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"test": {Name: "test", Host: "10.0.0.1"}},
		},
	}
	keymgr := &mockKeyManager{}
	svc := NewKeyService(keymgr, repo, &mockConnectorWithDeployFail{})

	err := svc.GenerateAndDeploy("test", "ed25519", 0, "")
	if err != nil {
		t.Fatalf("GenerateAndDeploy should succeed despite deploy failure: %v", err)
	}

	// 验证配置已更新
	server, _ := repo.Get("test")
	if server.Auth == nil || server.Auth.KeyFile != "/tmp/keys/abc123.pem" {
		t.Errorf("key file should be configured")
	}
}

// TestGenerateAndDeploy_DeployError 验证部署失败不影响主流程（密钥仍生成并配置成功）。
// 注意：新实现中 DeployService.DeployToServer 的错误不影响 GenerateAndDeploy 流程。
func TestGenerateAndDeploy_DeployError(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"test": {Name: "test", Host: "10.0.0.1"}},
		},
	}
	keymgr := &mockKeyManager{}
	svc := NewKeyService(keymgr, repo, &mockConnectorWithDeployFail{})

	err := svc.GenerateAndDeploy("test", "rsa", 4096, "")
	// 生成成功，部署失败不影响主流程
	if err != nil {
		t.Fatalf("GenerateAndDeploy should succeed despite deploy errors: %v", err)
	}

	// 验证配置已更新
	server, _ := repo.Get("test")
	if server.Auth == nil || server.Auth.KeyFile == "" {
		t.Errorf("key file should be configured despite deploy errors")
	}
}

// TestGenerateAndDeploy_UpdateConfigFails 验证配置更新失败时返回错误。
func TestGenerateAndDeploy_UpdateConfigFails(t *testing.T) {
	// 使用一个会在 Set 时返回错误的 mockRepo
	repo := &mockRepoFailSet{
		servers: map[string]map[string]*domain.Server{
			"": {"test": {Name: "test", Host: "10.0.0.1"}},
		},
	}
	keymgr := &mockKeyManager{}
	svc := NewKeyService(keymgr, repo, &mockConnectorWithDeployFail{})

	err := svc.GenerateAndDeploy("test", "rsa", 4096, "")
	if err == nil {
		t.Fatal("expected error when Set fails")
	}
}

// mockRepoFailSet 模拟 Set 操作失败的 mockRepo。
type mockRepoFailSet struct {
	servers map[string]map[string]*domain.Server
}

func (m *mockRepoFailSet) List() (map[string]map[string]*domain.Server, error) {
	return m.servers, nil
}

func (m *mockRepoFailSet) Get(name string) (*domain.Server, error) {
	for _, group := range m.servers {
		if s, ok := group[name]; ok {
			return s, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockRepoFailSet) Set(name string, server *domain.Server) error {
	return errors.New("database write failed")
}

func (m *mockRepoFailSet) Delete(name string) error {
	return nil
}

func (m *mockRepoFailSet) Move(from, to string) error {
	return nil
}

func (m *mockRepoFailSet) Search(keyword string) (map[string]map[string]*domain.Server, error) {
	return m.servers, nil
}

func (m *mockRepoFailSet) GetGroup(group string) (map[string]*domain.Server, error) {
	return m.servers[group], nil
}

func (m *mockRepoFailSet) GetChangelog(name string) ([]domain.ChangelogEntry, error) {
	return nil, domain.ErrNotFound
}

func (m *mockRepoFailSet) RollbackTo(name string, version int) error {
	return domain.ErrNotFound
}

func (m *mockRepoFailSet) Close() error { return nil }

// ---------------------------------------------------------------------------
// HandleKeyFlag 测试
// ---------------------------------------------------------------------------

// TestHandleKeyFlag_EmptyName 验证空服务器名返回 ErrInvalidName。
func TestHandleKeyFlag_EmptyName(t *testing.T) {
	svc := NewKeyService(&mockKeyManager{}, &mockRepo{}, &mockConnector{})

	_, err := svc.HandleKeyFlag("", "-k-value")
	if err != domain.ErrInvalidName {
		t.Errorf("expected ErrInvalidName, got %v", err)
	}
}

// TestHandleKeyFlag_ServerNotFound 验证不存在的服务器返回错误。
func TestHandleKeyFlag_ServerNotFound(t *testing.T) {
	svc := NewKeyService(&mockKeyManager{}, &mockRepo{}, &mockConnector{})

	_, err := svc.HandleKeyFlag("nonexist", "")
	if err == nil {
		t.Fatal("expected error for nonexistent server")
	}
}

// TestHandleKeyFlag_EmptyString 验证 keyValue="" 时触发 GenerateAndDeploy。
func TestHandleKeyFlag_EmptyString(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"test": {Name: "test", Host: "10.0.0.1"}},
		},
	}
	keymgr := &mockKeyManager{
		generateFunc: func(keyType string, bits int, passphrase []byte, comment string) (string, []byte, string, error) {
			if keyType != "rsa" {
				t.Errorf("expected rsa key type, got %q", keyType)
			}
			if bits != 4096 {
				t.Errorf("expected 4096 bits, got %d", bits)
			}
			return "/tmp/keys/gen.pem", []byte("ssh-rsa AAA..."), "gen123", nil
		},
		deployFunc: func(server *domain.Server, pubKey []byte) error {
			return nil // 成功部署
		},
	}
		svc := NewKeyService(keymgr, repo, &mockConnectorWithDeployFail{})

	keyPath, err := svc.HandleKeyFlag("test", "")
	if err != nil {
		t.Fatalf("HandleKeyFlag('') failed: %v", err)
	}
	if keyPath == "" {
		t.Error("keyPath should not be empty")
	}

	// 验证配置已更新
	server, _ := repo.Get("test")
	if server.Auth == nil || server.Auth.KeyFile == "" {
		t.Error("Auth.KeyFile should be set")
	}
}

// TestHandleKeyFlag_Dash 验证 keyValue="-" 时从 stdin 读取。
func TestHandleKeyFlag_Dash(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"test": {Name: "test", Host: "10.0.0.1"}},
		},
	}
	keymgr := &mockKeyManager{
		readStdinFunc: func() (string, string, error) {
			return "/tmp/keys/stdin_saved.pem", "stdin_fp", nil
		},
	}
		svc := NewKeyService(keymgr, repo, &mockConnectorWithDeployFail{})

	keyPath, err := svc.HandleKeyFlag("test", "-")
	if err != nil {
		t.Fatalf("HandleKeyFlag('-') failed: %v", err)
	}
	if keyPath != "/tmp/keys/stdin_saved.pem" {
		t.Errorf("expected /tmp/keys/stdin_saved.pem, got %q", keyPath)
	}

	// 验证配置已更新
	server, _ := repo.Get("test")
	if server.Auth == nil || server.Auth.KeyFile != "/tmp/keys/stdin_saved.pem" {
		t.Errorf("Auth.KeyFile should be updated from stdin")
	}
}

// TestHandleKeyFlag_StdinFails 验证 stdin 读取失败时返回错误。
func TestHandleKeyFlag_StdinFails(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"test": {Name: "test", Host: "10.0.0.1"}},
		},
	}
	keymgr := &mockKeyManager{
		readStdinFunc: func() (string, string, error) {
			return "", "", errors.New("empty input from stdin")
		},
	}
		svc := NewKeyService(keymgr, repo, &mockConnectorWithDeployFail{})

	_, err := svc.HandleKeyFlag("test", "-")
	if err == nil {
		t.Fatal("expected error when stdin read fails")
	}
}

// TestHandleKeyFlag_FilePath 验证 keyValue=文件路径时备份并配置。
func TestHandleKeyFlag_FilePath(t *testing.T) {
	// 创建临时密钥文件用于测试
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "test_key.pem")
	if err := os.WriteFile(keyFile, []byte("-----BEGIN OPENSSH PRIVATE KEY-----\ntest\n-----END OPENSSH PRIVATE KEY-----"), 0600); err != nil {
		t.Fatalf("write temp key file: %v", err)
	}

	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"test": {Name: "test", Host: "10.0.0.1"}},
		},
	}
	backupPath := filepath.Join(tmpDir, "backup.pem")
	keymgr := &mockKeyManager{
		backupPathFunc: func(name string) (string, bool) {
			return backupPath, false
		},
		backupFunc: func(srcPath, destPath string) (string, error) {
			if srcPath != keyFile {
				t.Errorf("expected src %q, got %q", keyFile, srcPath)
			}
			if destPath != backupPath {
				t.Errorf("expected dest %q, got %q", backupPath, destPath)
			}
			return "backup_fp", nil
		},
	}
		svc := NewKeyService(keymgr, repo, &mockConnectorWithDeployFail{})

	keyPath, err := svc.HandleKeyFlag("test", keyFile)
	if err != nil {
		t.Fatalf("HandleKeyFlag(path) failed: %v", err)
	}
	if keyPath != backupPath {
		t.Errorf("expected keyPath %q, got %q", backupPath, keyPath)
	}

	// 验证配置已更新
	server, _ := repo.Get("test")
	if server.Auth == nil || server.Auth.KeyFile != backupPath {
		t.Errorf("Auth.KeyFile should be updated from backup")
	}
}

// TestHandleKeyFlag_FileNotFound 验证不存在的文件路径返回错误。
func TestHandleKeyFlag_FileNotFound(t *testing.T) {
	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"test": {Name: "test", Host: "10.0.0.1"}},
		},
	}
	svc := NewKeyService(&mockKeyManager{}, repo, &mockConnector{})

	_, err := svc.HandleKeyFlag("test", "/nonexistent/key.pem")
	if err == nil {
		t.Fatal("expected error for nonexistent key file")
	}
}

// TestHandleKeyFlag_BackupFails 验证备份失败时返回错误。
func TestHandleKeyFlag_BackupFails(t *testing.T) {
	// 创建临时密钥文件
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "test_key.pem")
	if err := os.WriteFile(keyFile, []byte("test-key-content"), 0600); err != nil {
		t.Fatalf("write temp key file: %v", err)
	}

	repo := &mockRepo{
		servers: map[string]map[string]*domain.Server{
			"": {"test": {Name: "test", Host: "10.0.0.1"}},
		},
	}
	keymgr := &mockKeyManager{
		backupPathFunc: func(name string) (string, bool) {
			return filepath.Join(tmpDir, "backup.pem"), false
		},
		backupFunc: func(srcPath, destPath string) (string, error) {
			return "", errors.New("permission denied")
		},
	}
		svc := NewKeyService(keymgr, repo, &mockConnectorWithDeployFail{})

	_, err := svc.HandleKeyFlag("test", keyFile)
	if err == nil {
		t.Fatal("expected error when backup fails")
	}
}

// TestHandleKeyFlag_UpdateConfigFails 验证配置更新失败时返回错误。
func TestHandleKeyFlag_UpdateConfigFails(t *testing.T) {
	repo := &mockRepoFailSet{
		servers: map[string]map[string]*domain.Server{
			"": {"test": {Name: "test", Host: "10.0.0.1"}},
		},
	}
	svc := NewKeyService(&mockKeyManager{}, repo, &mockConnector{})

	_, err := svc.HandleKeyFlag("test", "-")
	if err == nil {
		t.Fatal("expected error when Set fails after stdin read")
	}
}
