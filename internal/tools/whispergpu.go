package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"github.com/devalexandre/llamafile-ui/internal/paths"
)

// WhisperGPU is whisper.cpp built with GPU support (Vulkan on Linux and
// Windows, Metal on macOS). Upstream publishes no Linux binaries and the
// whisperfile in llamafile 0.10.x is CPU-only, so the app's own CI builds it
// and attaches it to the app releases.
const WhisperGPU Tool = "whisper-gpu"

// WhisperCppVersion is the whisper.cpp tag built by the release workflow.
const WhisperCppVersion = "v1.9.5"

const appReleasesAPI = "https://api.github.com/repos/devalexandre/llamafile-ui/releases/latest"

// WhisperGPUAssetName is the package for this OS in the app release.
func WhisperGPUAssetName() (string, bool) {
	switch goruntime.GOOS + "/" + goruntime.GOARCH {
	case "linux/amd64":
		return "whisper-gpu-linux-x64.zip", true
	case "windows/amd64":
		return "whisper-gpu-windows-x64.zip", true
	case "darwin/arm64":
		return "whisper-gpu-macos-arm64.zip", true
	}
	return "", false
}

// WhisperGPURelease finds the package in the latest app release.
func WhisperGPURelease(ctx context.Context) (Asset, string, error) {
	name, ok := WhisperGPUAssetName()
	if !ok {
		return Asset{}, "", errors.New(i18n.T("não há build do whisper.cpp com GPU para este sistema"))
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, appReleasesAPI, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Asset{}, "", errors.New(i18n.T("não foi possível consultar as releases do app no GitHub"))
	}
	defer resp.Body.Close()
	var data struct {
		Assets []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&data) != nil {
		return Asset{}, "", errors.New(i18n.T("não foi possível consultar as releases do app no GitHub"))
	}
	for _, a := range data.Assets {
		if a.Name == name {
			return Asset{URL: a.URL, Size: a.Size}, name, nil
		}
	}
	return Asset{}, "", errors.New(i18n.T("a release atual do app ainda não traz o whisper.cpp com GPU; atualize o app"))
}

func WhisperGPUDir() string { return filepath.Join(paths.BinDir(), string(WhisperGPU)) }

// WhisperGPUBinary returns the installed whisper-cli and its version.
func WhisperGPUBinary() (path, version string) {
	name := "whisper-cli"
	if goruntime.GOOS == "windows" {
		name += ".exe"
	}
	p := filepath.Join(WhisperGPUDir(), name)
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return "", ""
	}
	v, _ := os.ReadFile(filepath.Join(WhisperGPUDir(), "VERSION"))
	return p, strings.TrimSpace(string(v))
}

func InstallWhisperGPU(file string) error {
	return installNative(file, WhisperGPUDir(), "whisper-cli", WhisperCppVersion, "whisper.cpp")
}

func RemoveWhisperGPU() error { return os.RemoveAll(WhisperGPUDir()) }
