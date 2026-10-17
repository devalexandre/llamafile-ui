// Package tools finds, downloads and runs the executables published in
// llamafile releases: llamafile (LLMs), whisperfile (speech-to-text) and
// diffusionfile (image generation).
package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/sysproc"
)

type Tool string

const (
	Llamafile Tool = "llamafile"
	Whisper   Tool = "whisperfile"
	Diffusion Tool = "diffusionfile"

	fallbackVersion = "0.10.6"
	releasesAPI     = "https://api.github.com/repos/mozilla-ai/llamafile/releases/latest"
)

// All lists the tools the app can manage.
var All = []Tool{Llamafile, Whisper, Diffusion}

// DownloadKey identifies a tool download in the download manager.
func DownloadKey(t Tool) string { return "tool:" + string(t) }

type Asset struct {
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

type Release struct {
	Version   string           `json:"version"`
	Published string           `json:"published"`
	Assets    map[string]Asset `json:"assets"` // "llamafile", "llamafile-thin", "whisperfile", "diffusionfile"
}

func fileName(t Tool, version string, thin bool) string {
	name := string(t) + "-" + version
	if thin {
		name += "-thin"
	}
	if goruntime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func assetKey(t Tool, thin bool) string {
	if thin {
		return string(t) + "-thin"
	}
	return string(t)
}

// Find looks for a tool: configured path, app bin dir (newest), then PATH.
func Find(t Tool, configured string) (path, version string) {
	if configured != "" {
		if st, err := os.Stat(configured); err == nil && !st.IsDir() {
			return configured, VersionFromName(t, filepath.Base(configured))
		}
	}
	entries, _ := os.ReadDir(paths.BinDir())
	var candidates []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, string(t)+"-") && !strings.HasSuffix(n, ".part") {
			candidates = append(candidates, n)
		}
	}
	if len(candidates) > 0 {
		// Newest version first; for the same version prefer the full (GPU) build.
		sort.Slice(candidates, func(i, j int) bool {
			vi, vj := VersionFromName(t, candidates[i]), VersionFromName(t, candidates[j])
			if vi != vj {
				return compareVersions(vi, vj) > 0
			}
			return !strings.Contains(candidates[i], "thin")
		})
		return filepath.Join(paths.BinDir(), candidates[0]), VersionFromName(t, candidates[0])
	}
	if p, err := exec.LookPath(string(t)); err == nil {
		return p, "sistema"
	}
	return "", ""
}

// IsThin reports whether a binary is the CPU-only "thin" build.
func IsThin(path string) bool { return strings.Contains(filepath.Base(path), "-thin") }

func VersionFromName(t Tool, name string) string {
	name = strings.TrimSuffix(strings.TrimPrefix(name, string(t)+"-"), ".exe")
	name = strings.TrimSuffix(name, "-thin")
	if name == "" || name == string(t) {
		return "personalizado"
	}
	return name
}

func compareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x - y
		}
	}
	return 0
}

func fallbackRelease() Release {
	base := "https://github.com/mozilla-ai/llamafile/releases/download/" + fallbackVersion + "/"
	return Release{Version: fallbackVersion, Assets: map[string]Asset{
		"llamafile":      {URL: base + "llamafile-" + fallbackVersion, Size: 368094430},
		"llamafile-thin": {URL: base + "llamafile-" + fallbackVersion + "-thin", Size: 43856395},
		"whisperfile":    {URL: base + "whisperfile-" + fallbackVersion, Size: 5467241},
		"diffusionfile":  {URL: base + "diffusionfile-" + fallbackVersion, Size: 73943040},
	}}
}

// LatestRelease asks GitHub for the newest llamafile release (with a fallback).
func LatestRelease(ctx context.Context) Release {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, releasesAPI, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fallbackRelease()
	}
	defer resp.Body.Close()
	var data struct {
		Tag       string `json:"tag_name"`
		Published string `json:"published_at"`
		Assets    []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&data) != nil || data.Tag == "" {
		return fallbackRelease()
	}
	rel := Release{Version: data.Tag, Published: data.Published, Assets: map[string]Asset{}}
	for _, a := range data.Assets {
		for _, t := range All {
			for _, thin := range []bool{false, true} {
				if a.Name == string(t)+"-"+data.Tag+map[bool]string{true: "-thin"}[thin] {
					rel.Assets[assetKey(t, thin)] = Asset{URL: a.URL, Size: a.Size}
				}
			}
		}
	}
	if _, ok := rel.Assets["llamafile"]; !ok {
		return fallbackRelease()
	}
	return rel
}

// Target returns the URL, destination and size to download a tool.
func Target(rel Release, t Tool, thin bool) (url, dest string, size int64, ok bool) {
	a, ok := rel.Assets[assetKey(t, thin)]
	if !ok {
		return "", "", 0, false
	}
	return a.URL, filepath.Join(paths.BinDir(), fileName(t, rel.Version, thin)), a.Size, true
}

// Command builds an exec.Cmd for an APE binary. Running it via sh avoids
// binfmt_misc handlers (e.g. Wine) hijacking it on Linux.
func Command(ctx context.Context, bin string, args ...string) *exec.Cmd {
	var cmd *exec.Cmd
	if goruntime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, bin, args...)
	} else {
		cmd = exec.CommandContext(ctx, "sh", append([]string{bin}, args...)...)
	}
	sysproc.Prepare(cmd)
	cmd.Cancel = func() error {
		sysproc.Kill(cmd)
		return nil
	}
	return cmd
}

// Threads is a sensible thread count for CPU-bound tools.
func Threads() int {
	n := goruntime.NumCPU()
	if n > 4 {
		return n - 2
	}
	return max(1, n)
}
