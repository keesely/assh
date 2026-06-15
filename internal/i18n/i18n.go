package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// Default language constants
const (
	DefaultLang = "zh-CN"
	LangEnvKey  = "ASSH_LANG"
)

// Supported languages
var SupportedLanguages = map[string]bool{
	"zh-CN": true,
	"en-US": true,
}

// Global state
var (
	bundle     *i18n.Bundle
	localizer  *i18n.Localizer
	currentLang string
	once       sync.Once
	initErr    error
)

// Init initializes the i18n system
// langDir is the directory containing translation files (e.g., "i18n/")
func Init(langDir string) error {
	once.Do(func() {
		// Create bundle with default language
		bundle = i18n.NewBundle(language.Chinese)
		bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

		// Load translation files
		langs := []string{"zh-CN", "en-US"}
		for _, lang := range langs {
			filePath := fmt.Sprintf("%s/%s.yaml", langDir, lang)
			if _, err := bundle.LoadMessageFile(filePath); err != nil {
				// If file doesn't exist, continue (will use fallback)
				fmt.Fprintf(os.Stderr, "Warning: failed to load %s: %v\n", filePath, err)
			}
		}

		// Detect language from environment
		currentLang = detectLang()

		// Create localizer
		localizer = i18n.NewLocalizer(bundle, currentLang)
	})
	return initErr
}

// detectLang detects language from environment variables
// Priority: ASSH_LANG > LC_ALL > LANG > DefaultLang
func detectLang() string {
	// Check ASSH_LANG first
	if lang := os.Getenv(LangEnvKey); lang != "" {
		return normalizeLang(lang)
	}

	// Check LC_ALL
	if lang := os.Getenv("LC_ALL"); lang != "" {
		return normalizeLang(lang)
	}

	// Check LANG
	if lang := os.Getenv("LANG"); lang != "" {
		return normalizeLang(lang)
	}

	return DefaultLang
}

// normalizeLang converts locale string to supported language code
// Examples: "en_US.UTF-8" -> "en-US", "zh_CN" -> "zh-CN"
func normalizeLang(locale string) string {
	// Remove encoding suffix (e.g., ".UTF-8")
	if idx := strings.Index(locale, "."); idx != -1 {
		locale = locale[:idx]
	}

	// Convert underscore to hyphen
	locale = strings.ReplaceAll(locale, "_", "-")

	// Check if it's a supported language
	if SupportedLanguages[locale] {
		return locale
	}

	// Try to match prefix (e.g., "en" -> "en-US")
	prefix := strings.Split(locale, "-")[0]
	switch prefix {
	case "zh":
		return "zh-CN"
	case "en":
		return "en-US"
	}

	return DefaultLang
}

// T returns translated text for the given key
// Args can be map[string]interface{} for template variables or positional args
func T(key string, args ...interface{}) string {
	if localizer == nil {
		// Fallback if not initialized
		return key
	}

	// Build template data
	var templateData map[string]interface{}
	if len(args) > 0 {
		if data, ok := args[0].(map[string]interface{}); ok {
			templateData = data
		} else {
			// Positional args: convert to map
			templateData = map[string]interface{}{
				"Arg0": args[0],
			}
		}
	}

	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: templateData,
	})
	if err != nil {
		// Fallback to key if translation not found
		return key
	}
	return msg
}

// TWithLang returns translated text for the given key using specified language
func TWithLang(lang, key string, args ...interface{}) string {
	if bundle == nil {
		return key
	}

	// Create temporary localizer
	tempLocalizer := i18n.NewLocalizer(bundle, lang)

	// Build template data
	var templateData map[string]interface{}
	if len(args) > 0 {
		if data, ok := args[0].(map[string]interface{}); ok {
			templateData = data
		} else {
			templateData = map[string]interface{}{
				"Arg0": args[0],
			}
		}
	}

	msg, err := tempLocalizer.Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: templateData,
	})
	if err != nil {
		return key
	}
	return msg
}

// GetCurrentLang returns the current language code
func GetCurrentLang() string {
	if currentLang == "" {
		return DefaultLang
	}
	return currentLang
}

// SetLang sets the current language
func SetLang(lang string) {
	currentLang = normalizeLang(lang)
	if bundle != nil {
		localizer = i18n.NewLocalizer(bundle, currentLang)
	}
}

// IsInitialized returns whether the i18n system has been initialized
func IsInitialized() bool {
	return bundle != nil
}

// SupportedLanguagesList returns a list of supported language codes
func SupportedLanguagesList() []string {
	langs := make([]string, 0, len(SupportedLanguages))
	for lang := range SupportedLanguages {
		langs = append(langs, lang)
	}
	return langs
}
