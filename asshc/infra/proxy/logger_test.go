package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"assh/asshc/port"
)

func TestProxyLogger_LogAndRead(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewProxyLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewProxyLogger failed: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	req := &port.RequestLog{
		SessionID:   "abc123",
		Timestamp:   now,
		Protocol:    "socks5",
		ClientAddr:  "127.0.0.1:54321",
		TargetAddr:  "www.google.com:443",
		Action:      "proxy",
		RuleMatched: "*.google.com",
		BytesSent:   1024,
		BytesRecv:   4096,
		Duration:    150 * time.Millisecond,
		Error:       "",
	}

	if err := logger.LogRequest(req); err != nil {
		t.Fatalf("LogRequest failed: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	dateStr := now.Format("2006-01-02")
	filePath := filepath.Join(tmpDir, dateStr, "requests.jsonl")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got := result["session_id"]; got != "abc123" {
		t.Errorf("session_id = %v, want abc123", got)
	}
	if got := result["protocol"]; got != "socks5" {
		t.Errorf("protocol = %v, want socks5", got)
	}
	if got := result["action"]; got != "proxy" {
		t.Errorf("action = %v, want proxy", got)
	}
	if got := result["rule_matched"]; got != "*.google.com" {
		t.Errorf("rule_matched = %v, want *.google.com", got)
	}
	if got := result["duration_ms"]; got != nil {
		dur := int64(got.(float64))
		if dur != 150 {
			t.Errorf("duration_ms = %d, want 150", dur)
		}
	}
}

func TestProxyLogger_MultipleEntries(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewProxyLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewProxyLogger failed: %v", err)
	}

	now := time.Now().UTC()
	count := 5
	for i := 0; i < count; i++ {
		req := &port.RequestLog{
			SessionID:  string(rune('a' + i)),
			Timestamp:  now,
			Protocol:   "socks5",
			ClientAddr: "127.0.0.1:54321",
			TargetAddr: "target.com:80",
			Action:     "proxy",
			BytesSent:  int64(i * 100),
			BytesRecv:  int64(i * 200),
			Duration:   time.Duration(i*50) * time.Millisecond,
		}
		if err := logger.LogRequest(req); err != nil {
			t.Fatalf("LogRequest %d failed: %v", i, err)
		}
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	dateStr := now.Format("2006-01-02")
	filePath := filepath.Join(tmpDir, dateStr, "requests.jsonl")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != count {
		t.Fatalf("got %d lines, want %d", len(lines), count)
	}

	for i, line := range lines {
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("line %d unmarshal failed: %v", i, err)
		}
		expectedSent := float64(i * 100)
		if got := entry["bytes_sent"]; got != expectedSent {
			t.Errorf("line %d bytes_sent = %v, want %v", i, got, expectedSent)
		}
	}
}

func TestProxyLogger_DailyRotation(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewProxyLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewProxyLogger failed: %v", err)
	}

	req := &port.RequestLog{
		SessionID:  "rotate-test",
		Timestamp:  time.Now().UTC(),
		Protocol:   "socks5",
		ClientAddr: "127.0.0.1:12345",
		TargetAddr: "example.com:443",
		Action:     "proxy",
		Duration:   100 * time.Millisecond,
	}
	if err := logger.LogRequest(req); err != nil {
		t.Fatalf("LogRequest failed: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	dirPath := filepath.Join(tmpDir, today)
	filePath := filepath.Join(dirPath, "requests.jsonl")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("log file not found at %s", filePath)
	}

	// Verify the directory naming convention is correct
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		t.Fatalf("log directory not found at %s", dirPath)
	}
}
