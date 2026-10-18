package main

import (
	"errors"
	"os"

	"github.com/devalexandre/llamafile-ui/internal/downloads"
	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"github.com/devalexandre/llamafile-ui/internal/tools"
)

// ToolStatus describes one of the executables from the llamafile release.
type ToolStatus struct {
	Tool        string `json:"tool"`
	Installed   bool   `json:"installed"`
	Path        string `json:"path"`
	Version     string `json:"version"`
	Thin        bool   `json:"thin"`
	Downloading bool   `json:"downloading"`
	// Compute backends detected in the build (whisperfile and diffusionfile).
	Backends []string `json:"backends"`
	GPU      bool     `json:"gpu"`
	Probed   bool     `json:"probed"`
}

func chmodExec(path string) error { return os.Chmod(path, 0o755) }

func (a *App) toolPath(t tools.Tool) string {
	configured := ""
	if t == tools.Llamafile {
		configured = a.settings.Get().LlamafilePath
	}
	path, _ := tools.Find(t, configured)
	return path
}

func (a *App) GetTools() []ToolStatus {
	out := make([]ToolStatus, 0, len(tools.All))
	for _, t := range tools.All {
		configured := ""
		if t == tools.Llamafile {
			configured = a.settings.Get().LlamafilePath
		}
		path, version := tools.Find(t, configured)
		s := ToolStatus{Tool: string(t), Installed: path != "", Path: path, Version: version, Backends: []string{},
			Thin: tools.IsThin(path), Downloading: a.downloads.IsActive(tools.DownloadKey(t))}
		if c, ok := a.toolGPU(t); ok && t != tools.Llamafile {
			s.Backends, s.GPU, s.Probed = c.Backends, c.GPU, true
		}
		out = append(out, s)
	}
	return append(out, a.whisperGPUStatus(), a.sdcppStatus(), a.piperStatus())
}

func (a *App) GetLatestRelease() tools.Release { return tools.LatestRelease(a.ctx) }

// DownloadTool fetches llamafile, whisperfile or diffusionfile from the latest
// release. thin only applies to llamafile (CPU-only build).
func (a *App) DownloadTool(name string, thin bool) error {
	t := tools.Tool(name)
	if t == piperTool {
		return a.downloadPiper()
	}
	if t == tools.SDCpp {
		return a.downloadSDCpp()
	}
	if t == tools.WhisperGPU {
		return a.downloadWhisperGPU()
	}
	rel := tools.LatestRelease(a.ctx)
	url, dest, size, ok := tools.Target(rel, t, thin)
	if !ok {
		return errors.New(i18n.Tf("%s não está disponível na versão %s", name, rel.Version))
	}
	key := tools.DownloadKey(t)
	a.mu.Lock()
	a.pendingTools[key] = dest
	a.mu.Unlock()
	label := name + " " + rel.Version
	if thin {
		label = i18n.Tf("%s %s (leve)", name, rel.Version)
	}
	a.emit("tools:changed")
	return a.downloads.Start(key, label, []downloads.File{{URL: url, Dest: dest, Size: size}})
}

// DownloadLlamafile is kept for the setup screen.
func (a *App) DownloadLlamafile(thin bool) error {
	return a.DownloadTool(string(tools.Llamafile), thin)
}

// DetectToolGPU re-checks the GPU backends of whisperfile and diffusionfile.
func (a *App) DetectToolGPU() []ToolStatus {
	a.detectToolGPU(true)
	return a.GetTools()
}
