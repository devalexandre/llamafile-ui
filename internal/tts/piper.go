// Package tts reads text aloud with Piper (https://github.com/rhasspy/piper):
// a small offline neural text-to-speech engine that runs on the CPU.
package tts

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

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
func Install(archive string) error {
	defer os.Remove(archive)
	tmp := Dir() + ".new"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	var err error
	if strings.HasSuffix(archive, ".zip") {
		err = extractZip(archive, tmp)
	} else {
		err = extractTarGz(archive, tmp)
	}
	if err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("não foi possível extrair o Piper: %w", err)
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
		return errors.New("o pacote do Piper não contém o executável")
	}
	return os.Chmod(Binary(), 0o755)
}

// target resolves an archive entry inside dest, rejecting path traversal.
func target(dest, name string) (string, error) {
	p := filepath.Join(dest, filepath.FromSlash(name))
	if !strings.HasPrefix(p, filepath.Clean(dest)+string(os.PathSeparator)) {
		return "", fmt.Errorf("caminho inválido no pacote: %s", name)
	}
	return p, nil
}

func extractTarGz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		p, err := target(dest, h.Name)
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(p, 0o755)
		case tar.TypeSymlink:
			// Shared libraries are symlinked (libfoo.so → libfoo.so.1); keep them relative.
			if filepath.IsAbs(h.Linkname) || strings.Contains(h.Linkname, "..") {
				return fmt.Errorf("link inválido no pacote: %s", h.Name)
			}
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			err = os.Symlink(h.Linkname, p)
		case tar.TypeReg:
			err = writeFile(p, tr, os.FileMode(h.Mode)&0o777)
		}
		if err != nil {
			return err
		}
	}
}

func extractZip(archive, dest string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		p, err := target(dest, zf.Name)
		if err != nil {
			return err
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		err = writeFile(p, rc, 0o755)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeFile(p string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
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
