// Package tts reads text aloud with Piper (https://github.com/rhasspy/piper):
// a small offline neural text-to-speech engine that runs on the CPU.
package tts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/devalexandre/llamafile-ui/internal/archive"
	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"github.com/devalexandre/llamafile-ui/internal/paths"
)

// Version is the Piper release with prebuilt binaries for every desktop OS.
const Version = "2023.11.14-2"

type Asset struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

var assets = map[string]Asset{
	"linux/amd64":   {Name: "piper_linux_x86_64.tar.gz", Size: 26460462},
	"linux/arm64":   {Name: "piper_linux_aarch64.tar.gz", Size: 26004717},
	"darwin/arm64":  {Name: "piper_macos_aarch64.tar.gz", Size: 19146957},
	"darwin/amd64":  {Name: "piper_macos_x64.tar.gz", Size: 19146927},
	"windows/amd64": {Name: "piper_windows_amd64.zip", Size: 22477236},
}

// ReleaseAsset is the Piper build for this OS/architecture.
func ReleaseAsset() (Asset, bool) {
	a, ok := assets[goruntime.GOOS+"/"+goruntime.GOARCH]
	if !ok {
		return Asset{}, false
	}
	a.URL = "https://github.com/rhasspy/piper/releases/download/" + Version + "/" + a.Name
	return a, true
}

// Dir is where Piper is installed (binary, libraries and espeak-ng-data).
func Dir() string { return filepath.Join(paths.BinDir(), "piper") }

// Binary returns the installed piper executable, or "".
func Binary() string {
	name := "piper"
	if goruntime.GOOS == "windows" {
		name += ".exe"
	}
	p := filepath.Join(Dir(), name)
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p
	}
	return ""
}

// Install extracts a downloaded Piper archive into Dir() and removes it.
// The archives contain a top-level "piper/" folder.
func Install(file string) error {
	defer os.Remove(file)
	tmp := Dir() + ".new"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	if err := archive.Extract(file, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf(i18n.T("não foi possível extrair o Piper: %w"), err)
	}
	root := filepath.Join(tmp, "piper")
	if _, err := os.Stat(root); err != nil {
		root = tmp // archive without the top-level folder
	}
	_ = os.RemoveAll(Dir())
	if err := os.Rename(root, Dir()); err != nil {
		return err
	}
	_ = os.RemoveAll(tmp)
	if Binary() == "" {
		return errors.New(i18n.T("o pacote do Piper não contém o executável"))
	}
	return os.Chmod(Binary(), 0o755)
}

// SampleRate reads the output rate from the voice config (<model>.json).
func SampleRate(model string) int {
	raw, err := os.ReadFile(model + ".json")
	if err == nil {
		var cfg struct {
			Audio struct {
				SampleRate int `json:"sample_rate"`
			} `json:"audio"`
		}
		if json.Unmarshal(raw, &cfg) == nil && cfg.Audio.SampleRate > 0 {
			return cfg.Audio.SampleRate
		}
	}
	return 22050
}
