package i18n

import (
	"os"
	"testing"
)

func TestNormalizeLang(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"en_US.UTF-8", "en-US"},
		{"zh_CN", "zh-CN"},
		{"zh-CN", "zh-CN"},
		{"en-US", "en-US"},
		{"en", "en-US"},
		{"zh", "zh-CN"},
		{"ja_JP.UTF-8", "zh-CN"}, // unsupported, fallback to default
		{"", "zh-CN"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeLang(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeLang(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDetectLang(t *testing.T) {
	// Save original env vars
	origASSH := os.Getenv("ASSH_LANG")
	origLC_ALL := os.Getenv("LC_ALL")
	origLANG := os.Getenv("LANG")
	defer func() {
		os.Setenv("ASSH_LANG", origASSH)
		os.Setenv("LC_ALL", origLC_ALL)
		os.Setenv("LANG", origLANG)
	}()

	// Test default
	os.Unsetenv("ASSH_LANG")
	os.Unsetenv("LC_ALL")
	os.Unsetenv("LANG")
	if got := detectLang(); got != DefaultLang {
		t.Errorf("detectLang() = %q, want %q", got, DefaultLang)
	}

	// Test ASSH_LANG
	os.Setenv("ASSH_LANG", "en_US.UTF-8")
	if got := detectLang(); got != "en-US" {
		t.Errorf("detectLang() = %q, want %q", got, "en-US")
	}

	// Test LC_ALL
	os.Unsetenv("ASSH_LANG")
	os.Setenv("LC_ALL", "zh_CN.UTF-8")
	if got := detectLang(); got != "zh-CN" {
		t.Errorf("detectLang() = %q, want %q", got, "zh-CN")
	}

	// Test LANG
	os.Unsetenv("LC_ALL")
	os.Setenv("LANG", "en_US.UTF-8")
	if got := detectLang(); got != "en-US" {
		t.Errorf("detectLang() = %q, want %q", got, "en-US")
	}
}

func TestGetAndSetLang(t *testing.T) {
	// Reset state
	currentLang = DefaultLang

	// Test Get
	if got := GetCurrentLang(); got != DefaultLang {
		t.Errorf("GetCurrentLang() = %q, want %q", got, DefaultLang)
	}

	// Test Set
	SetLang("en-US")
	if got := GetCurrentLang(); got != "en-US" {
		t.Errorf("GetCurrentLang() = %q, want %q", got, "en-US")
	}

	// Test Set with locale format
	SetLang("zh_CN.UTF-8")
	if got := GetCurrentLang(); got != "zh-CN" {
		t.Errorf("GetCurrentLang() = %q, want %q", got, "zh-CN")
	}
}

func TestSupportedLanguagesList(t *testing.T) {
	langs := SupportedLanguagesList()
	if len(langs) != 2 {
		t.Errorf("SupportedLanguagesList() returned %d languages, want 2", len(langs))
	}

	// Check both languages are present
	found := make(map[string]bool)
	for _, lang := range langs {
		found[lang] = true
	}
	if !found["zh-CN"] || !found["en-US"] {
		t.Errorf("SupportedLanguagesList() = %v, want zh-CN and en-US", langs)
	}
}

func TestIsInitialized(t *testing.T) {
	// Reset state
	bundle = nil
	if IsInitialized() {
		t.Error("IsInitialized() = true, want false before init")
	}
}
