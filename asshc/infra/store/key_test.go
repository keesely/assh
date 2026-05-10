package store

import (
	"encoding/base64"
	"testing"
)

func TestGenerateAndStoreKey(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if err := store.generateAndStoreKey(); err != nil {
		t.Fatalf("generateAndStoreKey failed: %v", err)
	}

	key, err := store.getCryptoKey()
	if err != nil {
		t.Fatalf("getCryptoKey failed: %v", err)
	}

	if len(key) != 32 {
		t.Errorf("key length = %d, want 32", len(key))
	}
}

func TestGetExistingKey(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	store1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	if err := store1.generateAndStoreKey(); err != nil {
		t.Fatalf("generateAndStoreKey failed: %v", err)
	}
	store1.Close()

	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store2.Close()

	key1, _ := store1.getCryptoKey()
	key2, err := store2.getCryptoKey()
	if err != nil {
		t.Fatalf("getCryptoKey failed: %v", err)
	}

	if string(key1) != string(key2) {
		t.Error("keys should be identical when loading existing key")
	}
}

func TestEncryptDecryptPassword(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	plaintext := "my-secret-password"

	encrypted, err := store.encryptPassword(plaintext)
	if err != nil {
		t.Fatalf("encryptPassword failed: %v", err)
	}

	if len(encrypted) == 0 {
		t.Error("encrypted should not be empty")
	}

	decrypted, err := store.decryptPassword(encrypted)
	if err != nil {
		t.Fatalf("decryptPassword failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecryptEmptyPassword(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	encrypted, err := store.encryptPassword("")
	if err != nil {
		t.Fatalf("encryptPassword failed: %v", err)
	}

	decrypted, err := store.decryptPassword(encrypted)
	if err != nil {
		t.Fatalf("decryptPassword failed: %v", err)
	}

	if decrypted != "" {
		t.Errorf("decrypted = %q, want empty string", decrypted)
	}
}

func TestKeyBase64Encoding(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	encoded := base64.StdEncoding.EncodeToString(key)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}

	if string(decoded) != string(key) {
		t.Error("key should roundtrip through base64 encoding")
	}
}