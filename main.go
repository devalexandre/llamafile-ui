package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/devalexandre/llamafile-ui/internal/paths"
)

//go:embed all:frontend/dist
var assets embed.FS

// version is set at build time with -ldflags "-X main.version=..."
var version = "dev"

//go:embed build/appicon.png
var icon []byte

// files serves chat attachments (/attachments/<name>) and generated images
// (/images/<name>) stored on disk.
func files(w http.ResponseWriter, r *http.Request) {
	dirs := map[string]func() string{"/attachments/": paths.AttachmentsDir, "/images/": paths.ImagesDir}
	for prefix, dir := range dirs {
		name, ok := strings.CutPrefix(r.URL.Path, prefix)
		if !ok || name == "" || strings.Contains(name, "..") {
			continue
		}
		http.ServeFile(w, r, filepath.Join(dir(), filepath.Base(name)))
		return
	}
	http.NotFound(w, r)
}

func main() {
	// WebKitGTK renders a blank window with the DMA-BUF renderer on many
	// NVIDIA setups ("Failed to create GBM buffer"). Respect a user override.
	if goruntime.GOOS == "linux" && os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") == "" {
		_ = os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}

	app, err := NewApp()
	if err != nil {
		log.Fatal(err)
	}

	err = wails.Run(&options.App{
		Title:     "Llamafile",
		Width:     1240,
		Height:    820,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: http.HandlerFunc(files),
		},
		BackgroundColour: &options.RGBA{R: 38, G: 40, B: 51, A: 255}, // Nocturne dark background
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		Bind: []interface{}{app},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "Llamafile",
				Message: "Rode modelos de IA localmente com llamafile.\nVersão " + version,
				Icon:    icon,
			},
		},
		Linux: &linux.Options{
			Icon:        icon,
			ProgramName: "llamafile-ui",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
