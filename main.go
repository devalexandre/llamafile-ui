package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/devalexandre/llamafile-ui/internal/eventbus"
	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

//go:embed build/tray.png
var trayIcon []byte

//go:embed build/tray-template.png
var trayTemplateIcon []byte

// files serves the event bus, chat attachments (/attachments/<name>) and
// generated images (/images/<name>); everything else goes to the embedded frontend.
func files(bus http.Handler) application.Middleware {
	return func(next http.Handler) http.Handler {
		dirs := map[string]func() string{"/attachments/": paths.AttachmentsDir, "/images/": paths.ImagesDir}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == eventbus.Path {
				bus.ServeHTTP(w, r)
				return
			}
			for prefix, dir := range dirs {
				name, ok := strings.CutPrefix(r.URL.Path, prefix)
				if !ok || name == "" || strings.Contains(name, "..") {
					continue
				}
				http.ServeFile(w, r, filepath.Join(dir(), filepath.Base(name)))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func main() {
	// WebKitGTK renders a blank window with the DMA-BUF renderer on many
	// NVIDIA setups ("Failed to create GBM buffer"). Respect a user override.
	if goruntime.GOOS == "linux" && os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") == "" {
		_ = os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}

	svc, err := NewApp()
	if err != nil {
		log.Fatal(err)
	}

	var quitting atomic.Bool
	var window *application.WebviewWindow
	show := func() {
		if window != nil {
			window.Show()
			window.Focus()
		}
	}

	app := application.New(application.Options{
		Name:        "Llamafile UI",
		Description: "Chat, imagens, voz e documentos com IA local usando llamafile",
		Icon:        icon,
		Services:    []application.Service{application.NewService(svc)},
		// Any quit path (tray "Sair", SIGTERM, logout) must not be turned into
		// "hide to tray" by the window-closing hook below.
		ShouldQuit: func() bool {
			quitting.Store(true)
			return true
		},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: files(svc.bus.Handler()),
		},
		// Two instances would fight over the llamafile port: focus the open one instead.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "io.github.devalexandre.llamafileui",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { show() },
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Linux: application.LinuxOptions{
			ProgramName:                   "llamafile-ui",
			DisableQuitOnLastWindowClosed: true,
		},
	})

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Llamafile UI",
		Width:            1240,
		Height:           820,
		MinWidth:         900,
		MinHeight:        600,
		URL:              "/",
		EnableFileDrop:   true,
		BackgroundColour: application.NewRGB(38, 40, 51), // Nocturne dark background
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 52,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
	})
	svc.window = window

	// Files dropped on elements marked with data-file-drop-target.
	window.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		svc.emit("files:dropped", e.Context().DroppedFiles())
	})

	// Closing the window keeps the app (and the llamafile server) in the tray.
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if !quitting.Load() && svc.settings.Get().CloseToTray {
			e.Cancel()
			window.Hide()
		}
	})

	tray := app.SystemTray.New()
	if goruntime.GOOS == "darwin" {
		tray.SetTemplateIcon(trayTemplateIcon)
	} else {
		tray.SetIcon(trayIcon)
	}
	tray.SetTooltip("Llamafile UI")

	menu := application.NewMenu()
	menu.Add("Abrir Llamafile UI").OnClick(func(*application.Context) { show() })
	menu.Add("Nova conversa").OnClick(func(*application.Context) {
		show()
		svc.emit("app:new-chat")
	})
	menu.AddSeparator()
	status := menu.Add("llamafile parado").SetEnabled(false)
	stop := menu.Add("Parar o servidor").SetEnabled(false)
	stop.OnClick(func(*application.Context) { svc.StopServer() })
	menu.AddSeparator()
	menu.Add("Sair").OnClick(func(*application.Context) {
		quitting.Store(true)
		app.Quit()
	})
	tray.SetMenu(menu)
	tray.OnClick(show)

	// Keep the tray menu in sync with the chat server.
	svc.onRuntime = func(s runtime.Status) {
		label := "llamafile parado"
		switch s.State {
		case runtime.StateRunning:
			label = "● " + s.ModelName
		case runtime.StateStarting:
			label = "Carregando " + s.ModelName + "…"
		case runtime.StateMissing:
			label = "llamafile não instalado"
		case runtime.StateError:
			label = "Erro no servidor"
		}
		status.SetLabel(label)
		stop.SetEnabled(s.State == runtime.StateRunning || s.State == runtime.StateStarting)
		menu.Update()
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
