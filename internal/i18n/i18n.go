// Package i18n translates user-facing text. Messages are written in
// Brazilian Portuguese and used as keys (gettext style): T("Configurações")
// returns the translation for the current language, falling back to English
// and then to the Portuguese source. The same locales/*.json files are bundled
// into the frontend.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

// Source is the language the messages are written in.
const Source = "pt-BR"

// Language is a supported UI language.
type Language struct {
	Code string `json:"code"`
	Name string `json:"name"` // native name
	RTL  bool   `json:"rtl"`
}

// Languages are the most spoken languages in the world, plus Japanese and German.
var Languages = []Language{
	{"en", "English", false},
	{"zh", "中文（简体）", false},
	{"hi", "हिन्दी", false},
	{"es", "Español", false},
	{"ar", "العربية", true},
	{"fr", "Français", false},
	{"bn", "বাংলা", false},
	{"pt-BR", "Português (Brasil)", false},
	{"ru", "Русский", false},
	{"id", "Bahasa Indonesia", false},
	{"ja", "日本語", false},
	{"de", "Deutsch", false},
}

//go:embed locales/*.json
var files embed.FS

var (
	mu      sync.RWMutex
	current = Source
	dicts   = map[string]map[string]string{}
)

func init() {
	for _, l := range Languages {
		if l.Code == Source {
			continue
		}
		raw, err := files.ReadFile("locales/" + l.Code + ".json")
		if err != nil {
			continue
		}
		d := map[string]string{}
		if json.Unmarshal(raw, &d) == nil {
			dicts[l.Code] = d
		}
	}
}

// Normalize maps a locale ("en_US.UTF-8", "zh-TW", "pt") to a supported code.
func Normalize(locale string) string {
	l := strings.ToLower(strings.TrimSpace(locale))
	l = strings.SplitN(l, ".", 2)[0]
	l = strings.ReplaceAll(l, "_", "-")
	if l == "" || l == "c" || l == "posix" {
		return "en"
	}
	base := strings.SplitN(l, "-", 2)[0]
	if base == "pt" {
		return "pt-BR"
	}
	for _, lang := range Languages {
		if lang.Code == base {
			return lang.Code
		}
	}
	return "en"
}

// Detect guesses the system language from the environment (Linux, macOS).
func Detect() string {
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"} {
		if s := os.Getenv(v); s != "" {
			return Normalize(strings.SplitN(s, ":", 2)[0])
		}
	}
	return "en"
}

// SetLanguage changes the current language ("" = detect).
func SetLanguage(code string) {
	if code == "" {
		code = Detect()
	}
	code = Normalize(code)
	mu.Lock()
	current = code
	mu.Unlock()
}

// Current returns the current language code.
func Current() string {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// T translates a message to the current language.
func T(msg string) string {
	mu.RLock()
	lang := current
	mu.RUnlock()
	return TIn(lang, msg)
}

// TIn translates a message to a given language.
func TIn(lang, msg string) string {
	if lang == Source {
		return msg
	}
	if s := dicts[lang][msg]; s != "" {
		return s
	}
	if s := dicts["en"][msg]; s != "" {
		return s
	}
	return msg
}

// Tf translates a format string and applies fmt.Sprintf.
func Tf(format string, args ...any) string { return fmt.Sprintf(T(format), args...) }

// N marks a message for translation without translating it (for tables
// translated later with T).
func N(msg string) string { return msg }
