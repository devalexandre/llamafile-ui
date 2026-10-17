package main

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/devalexandre/llamafile-ui/internal/downloads"
	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/tools"
)

// imageBinary resolves the image engine: the one chosen in Settings, or in
// "auto" the GPU build of stable-diffusion.cpp when installed, else diffusionfile.
func (a *App) imageBinary() (string, tools.Tool) {
	sd, _ := tools.SDCppBinary()
	df := a.toolPath(tools.Diffusion)
	switch a.settings.Get().ImageEngine {
	case "diffusionfile":
		if df != "" {
			return df, tools.Diffusion
		}
	case "sd-cpp":
		if sd != "" {
			return sd, tools.SDCpp
		}
	}
	if sd != "" {
		return sd, tools.SDCpp
	}
	if df != "" {
		return df, tools.Diffusion
	}
	return "", ""
}

func (a *App) sdcppStatus() ToolStatus {
	bin, version := tools.SDCppBinary()
	s := ToolStatus{Tool: string(tools.SDCpp), Installed: bin != "", Path: bin, Version: version, Backends: []string{},
		Downloading: a.downloads.IsActive(tools.DownloadKey(tools.SDCpp))}
	if c, ok := a.toolGPU(tools.SDCpp); ok {
		s.Backends, s.GPU, s.Probed = c.Backends, c.GPU, true
	}
	return s
}

func (a *App) downloadSDCpp() error {
	version, asset, name, err := tools.SDCppRelease(a.ctx)
	if err != nil {
		return err
	}
	key := tools.DownloadKey(tools.SDCpp)
	dest := filepath.Join(paths.BinDir(), name)
	a.mu.Lock()
	a.pendingTools[key] = dest
	a.mu.Unlock()
	a.emit("tools:changed")
	return a.downloads.Start(key, "stable-diffusion.cpp "+version, []downloads.File{{URL: asset.URL, Dest: dest, Size: asset.Size}})
}

// sdcppVersion derives the build id from the package name
// ("sd-master-f0aac52-bin-…zip" → "master-f0aac52").
func sdcppVersion(file string) string {
	name := strings.TrimPrefix(filepath.Base(file), "sd-")
	if i := strings.Index(name, "-bin-"); i > 0 {
		return name[:i]
	}
	return "desconhecida"
}

// ActiveImageEngine reports which engine generates images right now.
func (a *App) ActiveImageEngine() string {
	_, engine := a.imageBinary()
	return string(engine)
}

// RemoveTool deletes the GPU build of stable-diffusion.cpp.
func (a *App) RemoveTool(name string) error {
	if tools.Tool(name) != tools.SDCpp {
		return errors.New("só o stable-diffusion.cpp pode ser removido por aqui")
	}
	if err := tools.RemoveSDCpp(); err != nil {
		return err
	}
	a.emit("tools:changed")
	a.emit("catalog:changed")
	return nil
}
