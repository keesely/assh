package keymgr

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"assh/asshc/domain"

	"golang.org/x/crypto/ssh"
)

// setupKeyManager 创建临时目录和 KeyManager 实例供测试使用。
func setupKeyManager(t *testing.T, passphrase []byte) (*KeyManager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	km, err := New(tmpDir, passphrase)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return km, tmpDir
}

// createTestKeyFile 在指定目录创建测试密钥文件并返回其路径。
// content 为文件内容，mode 为权限。
func createTestKeyFile(t *testing.T, dir string, content []byte, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, "test_key.pem")
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatalf("create test key file: %v", err)
	}
	return path
}

// verifyKeyFile 验证密钥文件存在、权限正确、指纹匹配。
func verifyKeyFile(t *testing.T, path, expectedFingerprint string) {
	t.Helper()

	// 文件存在
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("key file not found at %s: %v", path, err)
	}

	// 权限 0600
	if info.Mode().Perm()&0077 != 0 {
		t.Errorf("key file permissions %o should be 0600 or stricter", info.Mode().Perm())
	}

	// 指纹匹配
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read key file: %v", err)
	}
	actualFingerprint := fmt.Sprintf("%x", sha256.Sum256(content))
	if actualFingerprint != expectedFingerprint {
		t.Errorf("fingerprint mismatch: got %q, want %q", actualFingerprint, expectedFingerprint)
	}
}

// verifyPublicKey 验证公钥可被 ssh 解析。
func verifyPublicKey(t *testing.T, pubKey []byte) {
	t.Helper()
	_, _, _, _, err := ssh.ParseAuthorizedKey(pubKey)
	if err != nil {
		t.Fatalf("parse authorized key: %v", err)
	}
}

// verifyPrivateKey 验证私钥可被 ssh 解析（无密码）。
func verifyPrivateKey(t *testing.T, privPath string) {
	t.Helper()
	content, err := os.ReadFile(privPath)
	if err != nil {
		t.Fatalf("read private key: %v", err)
	}
	_, err = ssh.ParsePrivateKey(content)
	if err != nil {
		t.Fatalf("parse private key (no passphrase): %v", err)
	}
}

// verifyPrivateKeyWithPassphrase 验证私钥可被 ssh 使用密码解析。
func verifyPrivateKeyWithPassphrase(t *testing.T, privPath string, passphrase []byte) {
	t.Helper()
	content, err := os.ReadFile(privPath)
	if err != nil {
		t.Fatalf("read private key: %v", err)
	}
	_, err = ssh.ParsePrivateKeyWithPassphrase(content, passphrase)
	if err != nil {
		t.Fatalf("parse private key with passphrase: %v", err)
	}
}

// ---------------------------------------------------------------------------
// New & Constructor
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()
	passphrase := []byte("test-passphrase")
	km, err := New(tmpDir, passphrase)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if km.keysDir != tmpDir {
		t.Errorf("keysDir = %q, want %q", km.keysDir, tmpDir)
	}

	// passphrase should be copied
	passphrase[0] = 'X'
	if km.passphrase[0] == 'X' {
		t.Error("passphrase was not copied, internal state affected by external modification")
	}

	// GetAccountPassphrase returns a copy
	pp := km.GetAccountPassphrase()
	pp[0] = 'Y'
	if km.passphrase[0] == 'Y' {
		t.Error("GetAccountPassphrase did not return a copy")
	}
}

func TestNew_NoPassphrase(t *testing.T) {
	tmpDir := t.TempDir()
	km, err := New(tmpDir, nil)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if pp := km.GetAccountPassphrase(); pp != nil {
		t.Errorf("expected nil passphrase, got %v", pp)
	}
}

func TestNew_ExpandPath(t *testing.T) {
	// 使用相对路径测试展开
	km, err := New("/tmp/test-keymgr-random", nil)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if !filepath.IsAbs(km.keysDir) {
		t.Errorf("keysDir should be absolute, got %q", km.keysDir)
	}
}

// ---------------------------------------------------------------------------
// GetAccountPassphrase
// ---------------------------------------------------------------------------

func TestGetAccountPassphrase(t *testing.T) {
	pass := []byte("secret-password")
	km, _ := setupKeyManager(t, pass)

	pp := km.GetAccountPassphrase()
	if string(pp) != "secret-password" {
		t.Errorf("GetAccountPassphrase = %q, want %q", pp, "secret-password")
	}

	// 修改返回值不应影响内部状态
	pp[0] = 'X'
	pp2 := km.GetAccountPassphrase()
	if string(pp2) != "secret-password" {
		t.Error("returned slice was not a copy")
	}
}

func TestGetAccountPassphrase_Nil(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	if pp := km.GetAccountPassphrase(); pp != nil {
		t.Errorf("expected nil, got %v", pp)
	}
}

// ---------------------------------------------------------------------------
// Generate — RSA
// ---------------------------------------------------------------------------

func TestGenerate_RSA(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	privPath, pubKey, fingerprint, err := km.Generate("rsa", 4096, nil, "")
	if err != nil {
		t.Fatalf("Generate RSA failed: %v", err)
	}

	verifyKeyFile(t, privPath, fingerprint)
	verifyPublicKey(t, pubKey)
	verifyPrivateKey(t, privPath)

	// 验证路径格式：三层嵌套目录 ab/cd/xxxxx.pem
	fpDir1 := fingerprint[:2]
	fpDir2 := fingerprint[2:4]
	expectedSuffix := fpDir1 + "/" + fpDir2 + "/" + fingerprint[4:] + ".pem"
	if !strings.HasSuffix(privPath, expectedSuffix) {
		t.Errorf("filename should end with %s: %s", expectedSuffix, privPath)
	}
}

func TestGenerate_RSA_DefaultBits(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	privPath, pubKey, fingerprint, err := km.Generate("rsa", 0, nil, "")
	if err != nil {
		t.Fatalf("Generate RSA with default bits failed: %v", err)
	}
	verifyKeyFile(t, privPath, fingerprint)
	verifyPublicKey(t, pubKey)
	verifyPrivateKey(t, privPath)
}

func TestGenerate_RSA_2048(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	privPath, pubKey, fingerprint, err := km.Generate("rsa", 2048, nil, "")
	if err != nil {
		t.Fatalf("Generate RSA 2048 failed: %v", err)
	}
	verifyKeyFile(t, privPath, fingerprint)
	verifyPublicKey(t, pubKey)
}

func TestGenerate_RSA_BitsTooSmall(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	_, _, _, err := km.Generate("rsa", 1024, nil, "")
	if err == nil {
		t.Fatal("expected error for RSA 1024, got nil")
	}
	if !strings.Contains(err.Error(), "at least 2048") {
		t.Errorf("error message should mention 2048, got: %v", err)
	}
}

func TestGenerate_RSA_InvalidBits(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	_, _, _, err := km.Generate("rsa", 3000, nil, "")
	if err == nil {
		t.Fatal("expected error for RSA 3000 (not multiple of 256), got nil")
	}
}

// ---------------------------------------------------------------------------
// Generate — Ed25519
// ---------------------------------------------------------------------------

func TestGenerate_Ed25519(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	privPath, pubKey, fingerprint, err := km.Generate("ed25519", 0, nil, "")
	if err != nil {
		t.Fatalf("Generate Ed25519 failed: %v", err)
	}

	verifyKeyFile(t, privPath, fingerprint)
	verifyPublicKey(t, pubKey)
	verifyPrivateKey(t, privPath)
}

func TestGenerate_Ed25519_IgnoredBits(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	// Ed25519 忽略 bits 参数
	privPath, pubKey, fingerprint, err := km.Generate("ed25519", 999, nil, "")
	if err != nil {
		t.Fatalf("Generate Ed25519 with bits=999 failed: %v", err)
	}
	verifyKeyFile(t, privPath, fingerprint)
	verifyPublicKey(t, pubKey)
}

// ---------------------------------------------------------------------------
// Generate — ECDSA
// ---------------------------------------------------------------------------

func TestGenerate_ECDSA(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	privPath, pubKey, fingerprint, err := km.Generate("ecdsa", 0, nil, "")
	if err != nil {
		t.Fatalf("Generate ECDSA failed: %v", err)
	}

	verifyKeyFile(t, privPath, fingerprint)
	verifyPublicKey(t, pubKey)
	verifyPrivateKey(t, privPath)
}

func TestGenerate_ECDSA_P256(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	privPath, pubKey, fingerprint, err := km.Generate("ecdsa", 256, nil, "")
	if err != nil {
		t.Fatalf("Generate ECDSA P-256 failed: %v", err)
	}
	verifyKeyFile(t, privPath, fingerprint)
	verifyPublicKey(t, pubKey)
}

func TestGenerate_ECDSA_P521(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	privPath, pubKey, fingerprint, err := km.Generate("ecdsa", 521, nil, "")
	if err != nil {
		t.Fatalf("Generate ECDSA P-521 failed: %v", err)
	}
	verifyKeyFile(t, privPath, fingerprint)
	verifyPublicKey(t, pubKey)
}

func TestGenerate_ECDSA_InvalidBits(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	_, _, _, err := km.Generate("ecdsa", 192, nil, "")
	if err == nil {
		t.Fatal("expected error for ECDSA 192, got nil")
	}
	if !strings.Contains(err.Error(), "256, 384, or 521") {
		t.Errorf("error message should mention valid sizes, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Generate — Invalid / Edge Cases
// ---------------------------------------------------------------------------

func TestGenerate_InvalidType(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	_, _, _, err := km.Generate("dsa", 2048, nil, "")
	if err == nil {
		t.Fatal("expected error for dsa key type, got nil")
	}
	if !strings.Contains(err.Error(), "supported") {
		t.Errorf("error should mention supported types, got: %v", err)
	}
}

func TestGenerate_WithPassphrase(t *testing.T) {
	pass := []byte("my-strong-passphrase")
	km, _ := setupKeyManager(t, nil)
	privPath, pubKey, fingerprint, err := km.Generate("rsa", 2048, pass, "")
	if err != nil {
		t.Fatalf("Generate RSA with passphrase failed: %v", err)
	}

	verifyKeyFile(t, privPath, fingerprint)
	verifyPublicKey(t, pubKey)

	// 使用密码解析
	verifyPrivateKeyWithPassphrase(t, privPath, pass)
}

func TestGenerate_FileIsInsideKeysDir(t *testing.T) {
	km, keysDir := setupKeyManager(t, nil)
	privPath, _, _, err := km.Generate("ed25519", 0, nil, "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// 确保文件在 keysDir 内
	rel, err := filepath.Rel(keysDir, privPath)
	if err != nil {
		t.Fatalf("file not relative to keysDir: %v", err)
	}
	if strings.HasPrefix(rel, "..") {
		t.Errorf("key file %s is outside keysDir %s", privPath, keysDir)
	}
}

// ---------------------------------------------------------------------------
// BackupKey
// ---------------------------------------------------------------------------

func TestBackupKey(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	content := []byte("-----BEGIN TEST KEY-----\nfoo\n-----END TEST KEY-----")
	srcPath := createTestKeyFile(t, t.TempDir(), content, 0600)

	destPath := filepath.Join(km.keysDir, "myserver.pem")
	fingerprint, err := km.BackupKey(srcPath, destPath)
	if err != nil {
		t.Fatalf("BackupKey failed: %v", err)
	}

	// 验证指纹
	expectedFP := fmt.Sprintf("%x", sha256.Sum256(content))
	if fingerprint != expectedFP {
		t.Errorf("fingerprint = %q, want %q", fingerprint, expectedFP)
	}

	// 验证备份文件存在
	verifyKeyFile(t, destPath, fingerprint)

	// 验证内容一致
	backedContent, _ := os.ReadFile(destPath)
	if !bytes.Equal(backedContent, content) {
		t.Error("backup content differs from source")
	}
}

func TestBackupKey_SameSourceToSameDest(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	content := []byte("-----BEGIN RSA PRIVATE KEY-----\ndata\n-----END RSA PRIVATE KEY-----")
	srcPath := createTestKeyFile(t, t.TempDir(), content, 0600)
	destPath := filepath.Join(km.keysDir, "myserver.pem")

	fp1, err := km.BackupKey(srcPath, destPath)
	if err != nil {
		t.Fatalf("first BackupKey failed: %v", err)
	}

	fp2, err := km.BackupKey(srcPath, destPath)
	if err != nil {
		t.Fatalf("second BackupKey failed: %v", err)
	}

	if fp1 != fp2 {
		t.Errorf("fingerprints should match: %q vs %q", fp1, fp2)
	}

	// 验证文件仍存在且指纹匹配
	verifyKeyFile(t, destPath, fp1)
}

func TestBackupKey_DifferentFilesSameDest(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()

	content1 := []byte("key content version 1")
	content2 := []byte("key content version 2")
	src1 := filepath.Join(tmpDir, "key1.pem")
	src2 := filepath.Join(tmpDir, "key2.pem")
	if err := os.WriteFile(src1, content1, 0600); err != nil {
		t.Fatalf("write key1: %v", err)
	}
	if err := os.WriteFile(src2, content2, 0600); err != nil {
		t.Fatalf("write key2: %v", err)
	}

	destPath := filepath.Join(km.keysDir, "myserver.pem")

	fp1, err := km.BackupKey(src1, destPath)
	if err != nil {
		t.Fatalf("BackupKey 1 failed: %v", err)
	}
	fp2, err := km.BackupKey(src2, destPath)
	if err != nil {
		t.Fatalf("BackupKey 2 failed: %v", err)
	}

	if fp1 == fp2 {
		t.Error("different files should produce different fingerprints")
	}

	// 最终文件应为 content2
	verifyKeyFile(t, destPath, fp2)
	savedContent, _ := os.ReadFile(destPath)
	if !bytes.Equal(savedContent, content2) {
		t.Error("dest should contain latest source content")
	}
}

func TestBackupKey_PermissionError(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	content := []byte("-----BEGIN TEST KEY-----\nloose permissions\n-----END TEST KEY-----")
	srcPath := createTestKeyFile(t, t.TempDir(), content, 0644)

	_, err := km.BackupKey(srcPath, filepath.Join(km.keysDir, "test.pem"))
	if err == nil {
		t.Fatal("expected permission error for 0644, got nil")
	}
	if !strings.Contains(err.Error(), "too permissive") {
		t.Errorf("error should mention permissions, got: %v", err)
	}
}

func TestBackupKey_NotExist(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	_, err := km.BackupKey("/nonexistent/path/key.pem", filepath.Join(km.keysDir, "test.pem"))
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestBackupKey_IsDir(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	_, err := km.BackupKey(t.TempDir(), filepath.Join(km.keysDir, "test.pem"))
	if err == nil {
		t.Fatal("expected error for directory path, got nil")
	}
}

func TestBackupKey_ReadOnlySource(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	content := []byte("-----BEGIN TEST KEY-----\nreadonly key\n-----END TEST KEY-----")
	srcPath := createTestKeyFile(t, t.TempDir(), content, 0400)
	destPath := filepath.Join(km.keysDir, "myserver.pem")

	fingerprint, err := km.BackupKey(srcPath, destPath)
	if err != nil {
		t.Fatalf("BackupKey with 0400 source failed: %v", err)
	}
	verifyKeyFile(t, destPath, fingerprint)
}

// ---------------------------------------------------------------------------
// BackupPath
// ---------------------------------------------------------------------------

func TestBackupPath_Found(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	content := []byte("-----BEGIN TEST KEY-----\nbackup path test\n-----END TEST KEY-----")
	srcPath := createTestKeyFile(t, t.TempDir(), content, 0600)

	serverName := "prod.web-01"
	expectedDest, _ := km.BackupPath(serverName)

	_, err := km.BackupKey(srcPath, expectedDest)
	if err != nil {
		t.Fatalf("BackupKey failed: %v", err)
	}

	// BackupPath should now find it
	path, found := km.BackupPath(serverName)
	if !found {
		t.Fatal("BackupPath should find backup after BackupKey")
	}
	if path != expectedDest {
		t.Errorf("path = %q, want %q", path, expectedDest)
	}

	// 文件确实存在
	if _, err := os.Stat(path); err != nil {
		t.Errorf("backup file at %s should exist: %v", path, err)
	}
}

func TestBackupPath_NotFound(t *testing.T) {
	km, _ := setupKeyManager(t, nil)

	path, found := km.BackupPath("nonexistent-server")
	if found {
		t.Fatal("BackupPath should return false for unbacked server")
	}
	if path == "" {
		t.Error("BackupPath should still return a path even when not found")
	}
	if !strings.HasSuffix(path, ".pem") {
		t.Errorf("path should end with .pem: %s", path)
	}
}

func TestBackupPath_DifferentServersDifferentPaths(t *testing.T) {
	km, _ := setupKeyManager(t, nil)

	path1, _ := km.BackupPath("server-a")
	path2, _ := km.BackupPath("server-b")

	if path1 == path2 {
		t.Error("different servers should produce different backup paths")
	}
}

func TestBackupPath_AfterOverwrite(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()

	serverName := "myserver"
	content := []byte("final key content")
	srcPath := createTestKeyFile(t, tmpDir, content, 0600)
	destPath, _ := km.BackupPath(serverName)

	// Backup twice with different content
	contentOld := []byte("old key content")
	srcOld := createTestKeyFile(t, tmpDir, contentOld, 0600)
	km.BackupKey(srcOld, destPath)

	// Now overwrite with new content
	fp, err := km.BackupKey(srcPath, destPath)
	if err != nil {
		t.Fatalf("BackupKey failed: %v", err)
	}

	// BackupPath should still find it, with latest content
	path, found := km.BackupPath(serverName)
	if !found {
		t.Fatal("BackupPath should still find after overwrite")
	}
	verifyKeyFile(t, path, fp)
}

// ---------------------------------------------------------------------------
// ReadKeyFromStdin
// ---------------------------------------------------------------------------

func TestReadKeyFromStdin(t *testing.T) {
	content := []byte("-----BEGIN TEST KEY-----\nstdin key content\n-----END TEST KEY-----")
	km, _ := setupKeyManager(t, nil)
	km.stdin = bytes.NewReader(content)

	savedPath, fingerprint, err := km.ReadKeyFromStdin()
	if err != nil {
		t.Fatalf("ReadKeyFromStdin failed: %v", err)
	}

	verifyKeyFile(t, savedPath, fingerprint)

	// 验证内容一致
	savedContent, _ := os.ReadFile(savedPath)
	if !bytes.Equal(savedContent, content) {
		t.Error("saved stdin content differs from input")
	}
}

func TestReadKeyFromStdin_Empty(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	km.stdin = bytes.NewReader([]byte{})

	_, _, err := km.ReadKeyFromStdin()
	if err == nil {
		t.Fatal("expected error for empty stdin, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// DeployPublicKey stub
// ---------------------------------------------------------------------------

func TestDeployPublicKey_NotImplemented(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	err := km.DeployPublicKey(&domain.Server{Name: "test"}, []byte("ssh-rsa AAAA..."))
	if err == nil {
		t.Fatal("DeployPublicKey should return error (not implemented)")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("error should mention not implemented, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Edge Cases
// ---------------------------------------------------------------------------

func TestNew_NonExistentDir(t *testing.T) {
	// 即使目录不存在，New 也应该成功（懒创建）
	km, err := New("/tmp/keymgr-test-nonexistent-"+fmt.Sprintf("%d", os.Getuid()), nil)
	if err != nil {
		t.Fatalf("New should not fail on nonexistent dir: %v", err)
	}
	if km.keysDir == "" {
		t.Error("keysDir should not be empty")
	}
}

func TestGenerate_AutoCreatesKeysDir(t *testing.T) {
	// 使用不存在的目录生成密钥，应该自动创建
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "deeply", "nested", "keys")
	km, err := New(nestedDir, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	_, _, _, err = km.Generate("ed25519", 0, nil, "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Errorf("keysDir should have been auto-created: %s", nestedDir)
	}
}

// ---------------------------------------------------------------------------
// GenerateToPath
// ---------------------------------------------------------------------------

func TestGenerateToPath_RSA(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test_rsa")

	pubKey, fingerprint, err := km.GenerateToPath("rsa", 4096, nil, "", outputPath)
	if err != nil {
		t.Fatalf("GenerateToPath RSA failed: %v", err)
	}

	verifyKeyFile(t, outputPath, fingerprint)
	verifyPrivateKey(t, outputPath)

	// 验证公钥文件存在且内容匹配
	pubPath := outputPath + ".pub"
	pubContent, err := os.ReadFile(pubPath)
	if err != nil {
		t.Fatalf("read public key: %v", err)
	}
	verifyPublicKey(t, pubContent)

	// 公钥文件有尾随换行符，返回的 pubKey 没有
	if !bytes.Equal(bytes.TrimSpace(pubContent), pubKey) {
		t.Error("public key file content differs from returned value")
	}
}

func TestGenerateToPath_RSA_DefaultBits(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rsa_default")

	_, fingerprint, err := km.GenerateToPath("rsa", 0, nil, "", outputPath)
	if err != nil {
		t.Fatalf("GenerateToPath RSA default bits failed: %v", err)
	}
	verifyKeyFile(t, outputPath, fingerprint)
}

func TestGenerateToPath_RSA_2048(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rsa_2048")

	_, fingerprint, err := km.GenerateToPath("rsa", 2048, nil, "", outputPath)
	if err != nil {
		t.Fatalf("GenerateToPath RSA 2048 failed: %v", err)
	}
	verifyKeyFile(t, outputPath, fingerprint)
}

func TestGenerateToPath_RSA_BitsTooSmall(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	_, _, err := km.GenerateToPath("rsa", 1024, nil, "", "/tmp/test_rsa_weak")
	if err == nil {
		t.Fatal("expected error for RSA 1024, got nil")
	}
	if !strings.Contains(err.Error(), "at least 2048") {
		t.Errorf("error should mention 2048, got: %v", err)
	}
}

func TestGenerateToPath_Ed25519(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test_ed25519")

	pubKey, fingerprint, err := km.GenerateToPath("ed25519", 0, nil, "", outputPath)
	if err != nil {
		t.Fatalf("GenerateToPath Ed25519 failed: %v", err)
	}

	verifyKeyFile(t, outputPath, fingerprint)
	verifyPrivateKey(t, outputPath)

	pubPath := outputPath + ".pub"
	pubContent, _ := os.ReadFile(pubPath)
	verifyPublicKey(t, pubContent)

	if !bytes.Equal(bytes.TrimSpace(pubContent), pubKey) {
		t.Error("public key file content differs from returned value")
	}
}

func TestGenerateToPath_Ed25519_IgnoredBits(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "ed25519_ignored")

	_, fingerprint, err := km.GenerateToPath("ed25519", 999, nil, "", outputPath)
	if err != nil {
		t.Fatalf("GenerateToPath Ed25519 with bits=999 failed: %v", err)
	}
	verifyKeyFile(t, outputPath, fingerprint)
}

func TestGenerateToPath_ECDSA(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test_ecdsa")

	pubKey, fingerprint, err := km.GenerateToPath("ecdsa", 384, nil, "", outputPath)
	if err != nil {
		t.Fatalf("GenerateToPath ECDSA failed: %v", err)
	}

	verifyKeyFile(t, outputPath, fingerprint)
	verifyPrivateKey(t, outputPath)

	pubPath := outputPath + ".pub"
	pubContent, _ := os.ReadFile(pubPath)
	verifyPublicKey(t, pubContent)
	if !bytes.Equal(bytes.TrimSpace(pubContent), pubKey) {
		t.Error("public key file content differs from returned value")
	}
}

func TestGenerateToPath_ECDSA_P256(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "ecdsa_p256")

	_, fingerprint, err := km.GenerateToPath("ecdsa", 256, nil, "", outputPath)
	if err != nil {
		t.Fatalf("GenerateToPath ECDSA P-256 failed: %v", err)
	}
	verifyKeyFile(t, outputPath, fingerprint)
}

func TestGenerateToPath_ECDSA_P521(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "ecdsa_p521")

	_, fingerprint, err := km.GenerateToPath("ecdsa", 521, nil, "", outputPath)
	if err != nil {
		t.Fatalf("GenerateToPath ECDSA P-521 failed: %v", err)
	}
	verifyKeyFile(t, outputPath, fingerprint)
}

func TestGenerateToPath_ECDSA_DefaultBits(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "ecdsa_default")

	_, fingerprint, err := km.GenerateToPath("ecdsa", 0, nil, "", outputPath)
	if err != nil {
		t.Fatalf("GenerateToPath ECDSA default bits failed: %v", err)
	}
	verifyKeyFile(t, outputPath, fingerprint)
	verifyPrivateKey(t, outputPath)
}

func TestGenerateToPath_ECDSA_InvalidBits(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	_, _, err := km.GenerateToPath("ecdsa", 192, nil, "", "/tmp/test_ecdsa_bad")
	if err == nil {
		t.Fatal("expected error for ECDSA 192, got nil")
	}
	if !strings.Contains(err.Error(), "256, 384, or 521") {
		t.Errorf("error should mention valid sizes, got: %v", err)
	}
}

func TestGenerateToPath_InvalidKeyType(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	_, _, err := km.GenerateToPath("dsa", 2048, nil, "", "/tmp/test_dsa")
	if err == nil {
		t.Fatal("expected error for dsa key type, got nil")
	}
	if !strings.Contains(err.Error(), "supported") {
		t.Errorf("error should mention supported types, got: %v", err)
	}
}

func TestGenerateToPath_DirectoryOutput(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()

	// 传递目录路径，应自动追加默认文件名
	_, fingerprint, err := km.GenerateToPath("ed25519", 0, nil, "", tmpDir)
	if err != nil {
		t.Fatalf("GenerateToPath to directory failed: %v", err)
	}

	expectedPriv := filepath.Join(tmpDir, "id_ed25519")
	verifyKeyFile(t, expectedPriv, fingerprint)
	verifyPrivateKey(t, expectedPriv)

	// 验证公钥文件
	pubPath := expectedPriv + ".pub"
	pubContent, _ := os.ReadFile(pubPath)
	verifyPublicKey(t, pubContent)
}

func TestGenerateToPath_TrailingSlashDirectory(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	dirWithSlash := tmpDir + "/"

	// 以 / 结尾的目录路径，应自动追加默认文件名
	_, fingerprint, err := km.GenerateToPath("rsa", 2048, nil, "", dirWithSlash)
	if err != nil {
		t.Fatalf("GenerateToPath to dir with trailing slash failed: %v", err)
	}

	expectedPriv := filepath.Join(tmpDir, "id_rsa")
	verifyKeyFile(t, expectedPriv, fingerprint)
}

func TestGenerateToPath_WithPassphrase(t *testing.T) {
	pass := []byte("test-passphrase")
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "encrypted_rsa")

	_, fingerprint, err := km.GenerateToPath("rsa", 2048, pass, "", outputPath)
	if err != nil {
		t.Fatalf("GenerateToPath with passphrase failed: %v", err)
	}

	verifyKeyFile(t, outputPath, fingerprint)

	// 无密码应无法解析
	content, _ := os.ReadFile(outputPath)
	_, err = ssh.ParsePrivateKey(content)
	if err == nil {
		t.Error("expected error parsing passphrase-protected key without passphrase")
	}

	// 有密码应能解析
	verifyPrivateKeyWithPassphrase(t, outputPath, pass)
}

func TestGenerateToPath_PublicKeyPermissions(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "perm_test")

	_, _, err := km.GenerateToPath("ed25519", 0, nil, "", outputPath)
	if err != nil {
		t.Fatalf("GenerateToPath failed: %v", err)
	}

	pubPath := outputPath + ".pub"
	info, err := os.Stat(pubPath)
	if err != nil {
		t.Fatalf("stat public key: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("public key permissions %o, want 0644", info.Mode().Perm())
	}

	// 私钥权限应为 0600
	privInfo, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat private key: %v", err)
	}
	if privInfo.Mode().Perm()&0077 != 0 {
		t.Errorf("private key permissions %o should be 0600 or stricter", privInfo.Mode().Perm())
	}
}

func TestGenerateToPath_AutoCreateParentDir(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "deeply", "nested", "keys")
	outputPath := filepath.Join(nestedDir, "auto_create_test")

	_, _, err := km.GenerateToPath("ed25519", 0, nil, "", outputPath)
	if err != nil {
		t.Fatalf("GenerateToPath with auto-create parent dir failed: %v", err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Errorf("output file should have been created: %s", outputPath)
	}
}

func TestGenerateToPath_OverwriteExisting(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "overwrite_test")

	// 第一次生成 RSA 密钥
	_, fp1, err := km.GenerateToPath("rsa", 2048, nil, "", outputPath)
	if err != nil {
		t.Fatalf("first GenerateToPath failed: %v", err)
	}

	// 第二次生成 Ed25519 密钥到同一路径（覆盖）
	_, fp2, err := km.GenerateToPath("ed25519", 0, nil, "", outputPath)
	if err != nil {
		t.Fatalf("second GenerateToPath failed: %v", err)
	}

	// 不同密钥类型指纹应不同
	if fp1 == fp2 {
		t.Error("different key types should produce different fingerprints")
	}

	// 文件现在应包含第二个密钥的内容
	verifyKeyFile(t, outputPath, fp2)
}

func TestGenerateToPath_RSA_DirectoryAutoNames(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()

	// 目录路径 → 自动使用 id_rsa
	_, _, err := km.GenerateToPath("rsa", 2048, nil, "", tmpDir)
	if err != nil {
		t.Fatalf("GenerateToPath RSA to directory failed: %v", err)
	}

	expected := filepath.Join(tmpDir, "id_rsa")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("expected auto-named file %s not found", expected)
	}
	expectedPub := expected + ".pub"
	if _, err := os.Stat(expectedPub); os.IsNotExist(err) {
		t.Errorf("expected auto-named public key %s not found", expectedPub)
	}
}

func TestGenerateToPath_ECDSA_DirectoryAutoNames(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()

	// 目录路径 → 自动使用 id_ecdsa
	_, _, err := km.GenerateToPath("ecdsa", 256, nil, "", tmpDir)
	if err != nil {
		t.Fatalf("GenerateToPath ECDSA to directory failed: %v", err)
	}

	expected := filepath.Join(tmpDir, "id_ecdsa")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("expected auto-named file %s not found", expected)
	}
}

func TestGenerateToPath_Ed25519_DirectoryAutoNames(t *testing.T) {
	km, _ := setupKeyManager(t, nil)
	tmpDir := t.TempDir()

	// 目录路径 → 自动使用 id_ed25519
	_, _, err := km.GenerateToPath("ed25519", 0, nil, "", tmpDir)
	if err != nil {
		t.Fatalf("GenerateToPath Ed25519 to directory failed: %v", err)
	}

	expected := filepath.Join(tmpDir, "id_ed25519")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("expected auto-named file %s not found", expected)
	}
}


