// Package settings persists user preferences as JSON.
package settings

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/devalexandre/llamafile-ui/internal/paths"
)

type Settings struct {
	Host          string  `json:"host"`
	Port          int     `json:"port"`
	LlamafilePath string  `json:"llamafilePath"`
	GPU           string  `json:"gpu"` // auto, nvidia, amd, apple, disable
	ContextSize   int     `json:"contextSize"`
	Threads       int     `json:"threads"` // 0 = auto
	AutoStart     bool    `json:"autoStart"`
	Theme         string  `json:"theme"` // system, light, dark
	ModelsDir     string  `json:"modelsDir"`
	SystemPrompt  string  `json:"systemPrompt"`
	Temperature   float64 `json:"temperature"`
	LastModelID   string  `json:"lastModelId"`
	AutoTitle     bool    `json:"autoTitle"`

	VoiceModelID  string `json:"voiceModelId"`
	VoiceLanguage string `json:"voiceLanguage"` // auto, pt, en…
	EmbedModelID  string `json:"embedModelId"`
	ImageModelID  string `json:"imageModelId"`
	FastDecode    bool   `json:"fastDecode"`  // TAESD for image decoding
	CloseToTray   bool   `json:"closeToTray"` // closing the window keeps the app in the system tray
}

func Defaults() Settings {
	return Settings{
		Host:          "127.0.0.1",
		Port:          8686,
		GPU:           "auto",
		ContextSize:   8192,
		AutoStart:     true,
		Theme:         "system",
		ModelsDir:     paths.DefaultModelsDir(),
		SystemPrompt:  "Você é um assistente útil, direto e preciso. Responda no idioma do usuário.",
		Temperature:   0.7,
		AutoTitle:     true,
		VoiceLanguage: "auto",
		FastDecode:    true,
		CloseToTray:   true,
	}
}

type Store struct {
	mu   sync.RWMutex
	path string
	s    Settings
}

func Load() *Store {
	st := &Store{path: paths.SettingsFile(), s: Defaults()}
	if data, err := os.ReadFile(st.path); err == nil {
		_ = json.Unmarshal(data, &st.s)
	}
	st.s = sanitize(st.s)
	_ = os.MkdirAll(st.s.ModelsDir, 0o755)
	return st
}

func sanitize(s Settings) Settings {
	d := Defaults()
	if s.Host == "" {
		s.Host = d.Host
	}
	if s.Port < 1024 || s.Port > 65535 {
		s.Port = d.Port
	}
	if s.GPU == "" {
		s.GPU = d.GPU
	}
	if s.ContextSize < 512 {
		s.ContextSize = d.ContextSize
	}
	if s.Threads < 0 {
		s.Threads = 0
	}
	if s.Theme == "" {
		s.Theme = d.Theme
	}
	if s.ModelsDir == "" {
		s.ModelsDir = d.ModelsDir
	}
	if s.VoiceLanguage == "" {
		s.VoiceLanguage = d.VoiceLanguage
	}
	if s.Temperature < 0 || s.Temperature > 2 {
		s.Temperature = d.Temperature
	}
	return s
}

func (st *Store) Get() Settings {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.s
}

// Save validates, persists and returns the stored settings.
func (st *Store) Save(s Settings) (Settings, error) {
	s = sanitize(s)
	st.mu.Lock()
	defer st.mu.Unlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return st.s, err
	}
	if err := os.WriteFile(st.path, data, 0o644); err != nil {
		return st.s, err
	}
	_ = os.MkdirAll(s.ModelsDir, 0o755)
	st.s = s
	return s, nil
}

// Update applies fn to a copy of the settings and saves it.
func (st *Store) Update(fn func(*Settings)) (Settings, error) {
	s := st.Get()
	fn(&s)
	return st.Save(s)
}
