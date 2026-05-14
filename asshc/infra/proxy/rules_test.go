package proxy

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempRuleFile(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.txt")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		if _, err := f.WriteString(line + "\n"); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()
	return path
}

func TestRuleEngine_DomainSuffix(t *testing.T) {
	path := writeTempRuleFile(t, []string{
		"! Test rules",
		"*.google.com",
		"*.youtube.com",
	})
	eng := NewRuleEngine(false)
	if err := eng.Load(path); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		target string
		proxy  bool
	}{
		{"www.google.com:443", true},
		{"mail.google.com:443", true},
		{"google.com:80", false},
		{"youtube.com:443", false},
		{"www.youtube.com:80", true},
		{"example.com:8080", false},
	}
	for _, tc := range tests {
		proxy, rule, err := eng.Match(tc.target)
		if err != nil {
			t.Errorf("Match(%q) error: %v", tc.target, err)
			continue
		}
		if proxy != tc.proxy {
			t.Errorf("Match(%q) = %v, want %v (rule=%+v)", tc.target, proxy, tc.proxy, rule)
		}
	}
}

func TestRuleEngine_ExactDomain(t *testing.T) {
	path := writeTempRuleFile(t, []string{
		"google.com",
		"twitter.com",
	})
	eng := NewRuleEngine(false)
	if err := eng.Load(path); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		target string
		proxy  bool
	}{
		{"google.com:443", true},
		{"www.google.com:443", false},
		{"twitter.com:80", true},
		{"api.twitter.com:443", false},
		{"example.com:22", false},
	}
	for _, tc := range tests {
		proxy, rule, err := eng.Match(tc.target)
		if err != nil {
			t.Errorf("Match(%q) error: %v", tc.target, err)
			continue
		}
		if proxy != tc.proxy {
			t.Errorf("Match(%q) = %v, want %v (rule=%+v)", tc.target, proxy, tc.proxy, rule)
		}
	}
}

func TestRuleEngine_CIDRMatching(t *testing.T) {
	path := writeTempRuleFile(t, []string{
		"10.0.0.0/8",
		"192.168.0.0/16",
		"172.16.0.0/12",
	})
	eng := NewRuleEngine(false)
	if err := eng.Load(path); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		target string
		proxy  bool
	}{
		{"10.0.0.1:80", true},
		{"10.255.255.255:443", true},
		{"192.168.1.1:22", true},
		{"192.168.0.100:8080", true},
		{"172.16.0.1:53", true},
		{"172.31.255.255:443", true},
		{"8.8.8.8:53", false},
		{"1.1.1.1:80", false},
		{"example.com:443", false},
	}
	for _, tc := range tests {
		proxy, rule, err := eng.Match(tc.target)
		if err != nil {
			t.Errorf("Match(%q) error: %v", tc.target, err)
			continue
		}
		if proxy != tc.proxy {
			t.Errorf("Match(%q) = %v, want %v (rule=%+v)", tc.target, proxy, tc.proxy, rule)
		}
	}
}

func TestRuleEngine_WhitelistPriority(t *testing.T) {
	path := writeTempRuleFile(t, []string{
		"*.google.com",
		"@@*.cn",
		"*.cn",
	})
	eng := NewRuleEngine(false)
	if err := eng.Load(path); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		target string
		proxy  bool
	}{
		{"www.google.com:80", true},
		{"mail.google.com:443", true},
		// @@*.cn whitelist should override *.cn
		{"www.baidu.cn:443", false},
		{"mail.qq.cn:80", false},
		// Non-matching
		{"example.com:22", false},
	}
	for _, tc := range tests {
		proxy, rule, err := eng.Match(tc.target)
		if err != nil {
			t.Errorf("Match(%q) error: %v", tc.target, err)
			continue
		}
		if proxy != tc.proxy {
			t.Errorf("Match(%q) = %v, want %v (rule=%+v)", tc.target, proxy, tc.proxy, rule)
		}
	}

	t.Run("whitelist_returns_direct_action", func(t *testing.T) {
		proxy, rule, err := eng.Match("www.baidu.cn:80")
		if err != nil {
			t.Fatal(err)
		}
		if proxy {
			t.Errorf("expected direct connection for whitelisted domain")
		}
		if rule == nil {
			t.Fatal("expected matched rule")
		}
		if rule.Action != "direct" {
			t.Errorf("expected action=direct, got=%s", rule.Action)
		}
		if rule.Pattern != "*.cn" {
			t.Errorf("expected pattern=*.cn, got=%s", rule.Pattern)
		}
	})
}

func TestRuleEngine_RegexPattern(t *testing.T) {
	path := writeTempRuleFile(t, []string{
		"/\\.gov\\.cn$/",
		"/^.*\\.corp\\.example\\.com$/",
	})
	eng := NewRuleEngine(false)
	if err := eng.Load(path); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		target string
		proxy  bool
	}{
		{"www.gov.cn:443", true},
		{"xxx.gov.cn:80", true},
		{"not-gov.cn:443", false},
		{"server.corp.example.com:8080", true},
		{"test.corp.example.com:443", true},
		{"other.com:80", false},
	}
	for _, tc := range tests {
		proxy, rule, err := eng.Match(tc.target)
		if err != nil {
			t.Errorf("Match(%q) error: %v", tc.target, err)
			continue
		}
		if proxy != tc.proxy {
			t.Errorf("Match(%q) = %v, want %v (rule=%+v)", tc.target, proxy, tc.proxy, rule)
		}
	}
}

func TestRuleEngine_DefaultProxy(t *testing.T) {
	path := writeTempRuleFile(t, []string{
		"! Only specific direct domains",
		"@@localhost",
		"@@*.internal",
	})
	eng := NewRuleEngine(true) // default = proxy
	if err := eng.Load(path); err != nil {
		t.Fatal(err)
	}

	if !eng.DefaultAction() {
		t.Error("DefaultAction() should return true (proxy)")
	}

	tests := []struct {
		target string
		proxy  bool
	}{
		{"google.com:443", true},
		{"example.com:80", true},
		{"localhost:8080", false},
		{"server.internal:22", false},
		{"random.host:443", true},
	}
	for _, tc := range tests {
		proxy, rule, err := eng.Match(tc.target)
		if err != nil {
			t.Errorf("Match(%q) error: %v", tc.target, err)
			continue
		}
		if proxy != tc.proxy {
			t.Errorf("Match(%q) = %v, want %v (rule=%+v)", tc.target, proxy, tc.proxy, rule)
		}
	}
}

func TestRuleEngine_DefaultDirect(t *testing.T) {
	path := writeTempRuleFile(t, []string{
		"*.google.com",
	})
	eng := NewRuleEngine(false) // default = direct
	if err := eng.Load(path); err != nil {
		t.Fatal(err)
	}

	if eng.DefaultAction() {
		t.Error("DefaultAction() should return false (direct)")
	}

	tests := []struct {
		target string
		proxy  bool
	}{
		{"www.google.com:443", true},
		{"example.com:80", false},
		{"random.org:22", false},
	}
	for _, tc := range tests {
		proxy, rule, err := eng.Match(tc.target)
		if err != nil {
			t.Errorf("Match(%q) error: %v", tc.target, err)
			continue
		}
		if proxy != tc.proxy {
			t.Errorf("Match(%q) = %v, want %v (rule=%+v)", tc.target, proxy, tc.proxy, rule)
		}
	}
}

func TestRuleEngine_NoMatchDefaultDirect(t *testing.T) {
	path := writeTempRuleFile(t, []string{
		"*.google.com",
		"10.0.0.0/8",
	})
	eng := NewRuleEngine(false)
	if err := eng.Load(path); err != nil {
		t.Fatal(err)
	}

	proxy, rule, err := eng.Match("unknown.host:9999")
	if err != nil {
		t.Fatal(err)
	}
	if proxy {
		t.Error("expected direct (false) for unmatched target with defaultDirect=false")
	}
	if rule != nil {
		t.Errorf("expected nil rule for unmatched target, got %+v", rule)
	}
}

func TestRuleEngine_LoadAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.txt")

	if err := os.WriteFile(path, []byte("*.google.com\n"), 0644); err != nil {
		t.Fatal(err)
	}

	eng := NewRuleEngine(false)
	if err := eng.Load(path); err != nil {
		t.Fatal(err)
	}

	proxy, _, err := eng.Match("www.google.com:80")
	if err != nil {
		t.Fatal(err)
	}
	if !proxy {
		t.Error("expected www.google.com to be proxy after initial load")
	}

	proxy, _, err = eng.Match("www.youtube.com:80")
	if err != nil {
		t.Fatal(err)
	}
	if proxy {
		t.Error("expected www.youtube.com to be direct (no rule yet)")
	}

	if err := os.WriteFile(path, []byte("*.google.com\n*.youtube.com\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := eng.Reload(); err != nil {
		t.Fatal(err)
	}

	proxy, _, err = eng.Match("www.youtube.com:80")
	if err != nil {
		t.Fatal(err)
	}
	if !proxy {
		t.Error("expected www.youtube.com to be proxy after reload")
	}

	proxy, _, err = eng.Match("www.google.com:80")
	if err != nil {
		t.Fatal(err)
	}
	if !proxy {
		t.Error("expected www.google.com to still be proxy after reload")
	}
}

func TestRuleEngine_ReloadWithoutLoad(t *testing.T) {
	eng := NewRuleEngine(false)
	if err := eng.Reload(); err == nil {
		t.Error("expected error when reloading without a prior load")
	}
}

func TestRuleEngine_InvalidRegex(t *testing.T) {
	// Invalid regex should be skipped without error
	path := writeTempRuleFile(t, []string{
		"*.google.com",
		"/[invalid regex/",
	})
	eng := NewRuleEngine(false)
	if err := eng.Load(path); err != nil {
		t.Fatal(err)
	}

	proxy, rule, err := eng.Match("www.google.com:80")
	if err != nil {
		t.Fatal(err)
	}
	if !proxy {
		t.Error("expected google.com match even with invalid regex in file")
	}
	if rule == nil {
		t.Fatal("expected matched rule")
	}
}

func TestRuleEngine_CommentLines(t *testing.T) {
	path := writeTempRuleFile(t, []string{
		"! This is a comment",
		"  ! indented comment",
		"*.google.com",
		"! another comment",
		"",
		"*.youtube.com",
	})
	eng := NewRuleEngine(false)
	if err := eng.Load(path); err != nil {
		t.Fatal(err)
	}

	proxy, _, err := eng.Match("www.google.com:80")
	if err != nil {
		t.Fatal(err)
	}
	if !proxy {
		t.Error("expected www.google.com to be proxy")
	}

	proxy, _, err = eng.Match("www.youtube.com:80")
	if err != nil {
		t.Fatal(err)
	}
	if !proxy {
		t.Error("expected www.youtube.com to be proxy")
	}

	proxy, _, err = eng.Match("example.com:80")
	if err != nil {
		t.Fatal(err)
	}
	if proxy {
		t.Error("expected example.com to be direct")
	}
}

func TestRuleEngine_LowercaseMatch(t *testing.T) {
	path := writeTempRuleFile(t, []string{
		"*.Google.COM",
		"Example.COM",
	})
	eng := NewRuleEngine(false)
	if err := eng.Load(path); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		target string
		proxy  bool
	}{
		{"WWW.GOOGLE.COM:443", true},
		{"www.google.com:80", true},
		{"Google.COM:443", false},
		{"example.COM:80", true},
		{"Example.Com:443", true},
	}
	for _, tc := range tests {
		proxy, rule, err := eng.Match(tc.target)
		if err != nil {
			t.Errorf("Match(%q) error: %v", tc.target, err)
			continue
		}
		if proxy != tc.proxy {
			t.Errorf("Match(%q) = %v, want %v (rule=%+v)", tc.target, proxy, tc.proxy, rule)
		}
	}
}
