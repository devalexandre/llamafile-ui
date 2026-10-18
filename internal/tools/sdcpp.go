package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/devalexandre/llamafile-ui/internal/archive"
	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"github.com/devalexandre/llamafile-ui/internal/paths"
)

// SDCpp is the upstream stable-diffusion.cpp build, which (unlike the
// diffusionfile in llamafile 0.10.x) ships GPU backends: Vulkan on Linux and
// Windows, Metal on Apple Silicon. It runs the same models with the same flags.
const SDCpp Tool = "sd-cpp"

const sdcppAPI = "https://api.github.com/repos/leejet/stable-diffusion.cpp/releases/latest"

// Pinned build used when GitHub cannot be reached.
const (
	sdcppFallbackTag  = "master-973-f0aac52"
	sdcppFallbackHash = "f0aac52"
)

// sdcppSuffix picks the package for this OS: Vulkan works with NVIDIA, AMD and
// Intel GPUs without the 500 MB CUDA runtime.
func sdcppSuffix() (string, bool) {
	switch goruntime.GOOS + "/" + goruntime.GOARCH {
	case "linux/amd64":
		return "-bin-Linux-Ubuntu-24.04-x86_64-vulkan.zip", true
	case "windows/amd64":
		return "-bin-win-vulkan-x64.zip", true
	case "darwin/arm64":
		return "-arm64.zip", true // Metal
	}
	return "", false
}

// SDCppRelease returns the download for this OS from the latest upstream release.
func SDCppRelease(ctx context.Context) (version string, asset Asset, name string, err error) {
	suffix, ok := sdcppSuffix()
	if !ok {
		return "", Asset{}, "", errors.New(i18n.T("não há build com GPU do stable-diffusion.cpp para este sistema"))
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, sdcppAPI, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	if resp, err := http.DefaultClient.Do(req); err == nil {
		defer resp.Body.Close()
		var data struct {
			Tag    string `json:"tag_name"`
			Assets []struct {
				Name string `json:"name"`
				Size int64  `json:"size"`
				URL  string `json:"browser_download_url"`
			} `json:"assets"`
		}
		if resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&data) == nil {
			for _, a := range data.Assets {
				if strings.HasPrefix(a.Name, "sd-") && strings.HasSuffix(a.Name, suffix) &&
					(goruntime.GOOS != "darwin" || strings.Contains(a.Name, "Darwin")) {
					return data.Tag, Asset{URL: a.URL, Size: a.Size}, a.Name, nil
				}
			}
		}
	}
	name = "sd-master-" + sdcppFallbackHash + suffix
	if goruntime.GOOS == "darwin" {
		name = "sd-master-" + sdcppFallbackHash + "-bin-Darwin-macOS-26.6.2-arm64.zip"
	}
	url := "https://github.com/leejet/stable-diffusion.cpp/releases/download/" + sdcppFallbackTag + "/" + name
	return sdcppFallbackTag, Asset{URL: url}, name, nil
}

// SDCppDir is where the build is installed (binary + its shared libraries).
func SDCppDir() string { return filepath.Join(paths.BinDir(), string(SDCpp)) }

// SDCppBinary returns the installed sd-cli executable and its version.
func SDCppBinary() (path, version string) {
	name := "sd-cli"
	if goruntime.GOOS == "windows" {
		name += ".exe"
	}
	p := filepath.Join(SDCppDir(), name)
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return "", ""
	}
	v, _ := os.ReadFile(filepath.Join(SDCppDir(), "VERSION"))
	return p, strings.TrimSpace(string(v))
}

// InstallSDCpp extracts a downloaded package into SDCppDir and removes it.
func InstallSDCpp(file, version string) error {
	return installNative(file, SDCppDir(), "sd-cli", version, "stable-diffusion.cpp")
}

// installNative extracts a package holding a native executable (plus its
// libraries) into dir, records its version and removes the package.
func installNative(file, dir, binName, version, label string) error {
	defer os.Remove(file)
	tmp := dir + ".new"
	_ = os.RemoveAll(tmp)
	if err := archive.Extract(file, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf(i18n.T("não foi possível extrair o %s: %w"), label, err)
	}
	// The binary may sit in a subfolder depending on the package.
	root := ""
	_ = filepath.WalkDir(tmp, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.TrimSuffix(d.Name(), ".exe") == binName {
			root = filepath.Dir(p)
			return fs.SkipAll
		}
		return nil
	})
	if root == "" {
		_ = os.RemoveAll(tmp)
		return errors.New(i18n.Tf("o pacote do %s não contém o %s", label, binName))
	}
	_ = os.WriteFile(filepath.Join(root, "VERSION"), []byte(version), 0o644)
	_ = os.RemoveAll(dir)
	if err := os.Rename(root, dir); err != nil {
		return err
	}
	_ = os.RemoveAll(tmp)
	name := binName
	if goruntime.GOOS == "windows" {
		name += ".exe"
	}
	return os.Chmod(filepath.Join(dir, name), 0o755)
}

// RemoveSDCpp deletes the installed build.
func RemoveSDCpp() error { return os.RemoveAll(SDCppDir()) }
