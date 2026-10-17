package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/devalexandre/llamafile-ui/internal/bench"
	"github.com/devalexandre/llamafile-ui/internal/catalog"
	"github.com/devalexandre/llamafile-ui/internal/chat"
	"github.com/devalexandre/llamafile-ui/internal/downloads"
	"github.com/devalexandre/llamafile-ui/internal/eventbus"
	"github.com/devalexandre/llamafile-ui/internal/hardware"
	"github.com/devalexandre/llamafile-ui/internal/imagegen"
	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/rag"
	"github.com/devalexandre/llamafile-ui/internal/runtime"
	"github.com/devalexandre/llamafile-ui/internal/settings"
	"github.com/devalexandre/llamafile-ui/internal/tools"
	"github.com/devalexandre/llamafile-ui/internal/tts"
	"github.com/devalexandre/llamafile-ui/internal/voice"
)

const modelKeyPrefix = "model:"

// App is the Wails service bound to the frontend: every exported method is
// callable from React. Methods are grouped by feature in app_*.go files; the
// interfaces used by internal packages are implemented by [backend] so they
// are not exposed to JavaScript.
type App struct {
	ctx       context.Context
	window    *application.WebviewWindow
	bus       *eventbus.Bus
	onRuntime func(runtime.Status) // tray menu hook
	settings  *settings.Store
	catalog   *catalog.Catalog
	downloads *downloads.Manager
	runtime   *runtime.Manager // chat server
	embedder  *runtime.Manager // embeddings server (port + 1)
	chat      *chat.Service
	docs      *rag.Store
	images    *imagegen.Generator
	voice     *voice.Store
	recorder  voice.Recorder
	bench     *bench.Store
	speaker   tts.Speaker

	hwOnce sync.Once
	hwMu   sync.RWMutex
	hw     hardware.Info

	mu              sync.Mutex
	pendingTools    map[string]string       // download key → destination
	chatImages      map[string]chatImageJob // image job → chat message waiting for it
	imagePending    map[string]bool         // conversations waiting for an image
	transcribeAbort context.CancelFunc
}

func NewApp() (*App, error) {
	a := &App{settings: settings.Load(), downloads: downloads.New(), pendingTools: map[string]string{}, chatImages: map[string]chatImageJob{}, imagePending: map[string]bool{}, bus: eventbus.New()}
	cat, err := catalog.New(func() string { return a.settings.Get().ModelsDir })
	if err != nil {
		return nil, err
	}
	a.catalog = cat
	a.bench = bench.Open(filepath.Join(paths.DataDir(), "benchmarks.json"), a.hardware())
	cat.SetCalibrator(a.bench.Calibrator())
	a.runtime = runtime.New(a.settings, cat, runtime.ChatRole)
	a.embedder = runtime.New(a.settings, cat, runtime.EmbedRole)

	store, err := chat.Open(paths.ChatDatabase())
	if err != nil {
		return nil, err
	}
	be := &backend{a}
	a.chat = chat.NewService(store, be, chat.Events{
		Message: func(m chat.Message) { a.emit("chat:message", m) },
		Delta: func(conv string, id int64, content, reasoning string) {
			a.emit("chat:delta", map[string]any{"conversationId": conv, "messageId": id, "content": content, "reasoning": reasoning})
		},
		Done: func(m chat.Message) {
			a.emit("chat:done", m)
			a.observeChat(m)
		},
		Conversations: func() { a.emit("chat:conversations") },
		Status: func(conv, status string) {
			a.emit("chat:status", map[string]string{"conversationId": conv, "status": status})
		},
	})

	db := store.DB()
	if a.voice, err = voice.OpenStore(db); err != nil {
		return nil, err
	}
	if a.docs, err = rag.Open(db, be, rag.Events{
		Document: func(d rag.Document) { a.emit("docs:document", d) },
		Changed:  func() { a.emit("docs:changed") },
	}); err != nil {
		return nil, err
	}
	if a.images, err = imagegen.New(db, paths.ImagesDir(), be, imagegen.Events{
		Progress: func(p imagegen.Progress) { a.emit("image:progress", p) },
		Done: func(job string, img imagegen.Image) {
			a.emit("image:done", map[string]any{"jobId": job, "image": img})
			a.observeImage(img)
			a.chatImageFinished(job, &img, "")
		},
		Error: func(job, msg string) {
			a.emit("image:error", map[string]string{"jobId": job, "error": msg})
			a.chatImageFinished(job, nil, msg)
		},
	}); err != nil {
		return nil, err
	}
	a.speaker.OnState = func(id string, playing bool, err string) {
		a.emit("tts:state", map[string]any{"id": id, "playing": playing, "error": err})
	}
	a.recorder.OnLevel = func(level float64, elapsed time.Duration) {
		a.emit("voice:level", map[string]float64{"level": level, "elapsed": elapsed.Seconds()})
	}
	return a, nil
}

// emit sends an event to the frontend through the long-poll event bus.
func (a *App) emit(event string, data ...any) {
	var payload any
	if len(data) > 0 {
		payload = data[0]
	}
	a.bus.Publish(event, payload)
}

// ServiceStartup is called by Wails when the application starts.
func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.ctx = ctx
	a.runtime.OnChange = func(s runtime.Status) {
		a.emit("runtime:status", s)
		if a.onRuntime != nil {
			a.onRuntime(s)
		}
	}
	a.runtime.OnLog = func(line string) { a.emit("runtime:log", line) }
	a.embedder.OnChange = func(s runtime.Status) { a.emit("embedder:status", s) }
	a.downloads.OnChange = func(s []downloads.Status) { a.emit("downloads:progress", s) }
	a.downloads.OnDone = a.downloadDone

	go func() {
		a.hardware()
		a.detectToolGPU(false)
		cfg := a.settings.Get()
		if cfg.AutoStart && cfg.LastModelID != "" && a.catalog.ModelPath(cfg.LastModelID) != "" &&
			a.runtime.Status().State == runtime.StateStopped {
			_ = a.runtime.Start(cfg.LastModelID)
		}
	}()
	return nil
}

// ServiceShutdown stops every child process before the app exits.
func (a *App) ServiceShutdown() error {
	a.chat.StopAll()
	a.images.Cancel()
	a.recorder.Cancel()
	a.speaker.Stop()
	a.CancelTranscription()
	a.runtime.Stop()
	a.embedder.Stop()
	return a.chat.Store().Close()
}

// ---------------------------------------------------------------- system

func (a *App) hardware() hardware.Info {
	a.hwOnce.Do(func() {
		info := hardware.Detect()
		a.hwMu.Lock()
		a.hw = info
		a.hwMu.Unlock()
	})
	a.hwMu.RLock()
	defer a.hwMu.RUnlock()
	return a.hw
}

// fitHardware is the hardware as llamafile will use it: the "thin" build has
// no GPU support, so recommendations must assume CPU.
func (a *App) fitHardware() hardware.Info {
	hw := a.hardware()
	if c, ok := a.toolGPU(tools.Whisper); ok {
		hw.WhisperGPU = c.GPU
	}
	if _, engine := a.imageBinary(); engine != "" {
		if c, ok := a.toolGPU(engine); ok {
			hw.DiffusionGPU = c.GPU
		}
	}
	// The thin llamafile only affects LLMs: image and voice tools may still use the GPU.
	hw.LLMCPUOnly = a.runtime.Status().Thin
	return hw
}

func (a *App) GetHardware() hardware.Info { return a.hardware() }

func (a *App) RefreshHardware() hardware.Info {
	info := hardware.Detect()
	a.hwMu.Lock()
	a.hw = info
	a.hwMu.Unlock()
	a.emit("catalog:changed")
	return info
}

func (a *App) GetDiskFreeGB() float64 { return hardware.DiskFreeGB(a.settings.Get().ModelsDir) }

func (a *App) GetPlatform() string { return goruntime.GOOS }

func (a *App) GetSettings() settings.Settings { return a.settings.Get() }

// updateSettings changes settings from the backend and tells the frontend, so
// its copy never goes stale (and never overwrites newer values on save).
func (a *App) updateSettings(fn func(*settings.Settings)) error {
	before := a.settings.Get()
	after := before
	fn(&after)
	if after == before {
		return nil
	}
	saved, err := a.settings.Save(after)
	if err == nil {
		a.emit("settings:changed", saved)
	}
	return err
}

func (a *App) SaveSettings(s settings.Settings) (settings.Settings, error) {
	before := a.settings.Get()
	saved, err := a.settings.Save(s)
	if err != nil {
		return saved, err
	}
	a.emit("settings:changed", saved)
	if saved.ModelsDir != before.ModelsDir || saved.ContextSize != before.ContextSize {
		a.emit("catalog:changed")
	}
	if saved.LlamafilePath != before.LlamafilePath {
		a.runtime.Detect()
		a.embedder.Detect()
	} else {
		a.emit("runtime:status", a.runtime.Status())
	}
	return saved, nil
}

func (a *App) ResetServerSettings() (settings.Settings, error) {
	d := settings.Defaults()
	s := a.settings.Get()
	s.Host, s.Port, s.GPU, s.ContextSize, s.Threads = d.Host, d.Port, d.GPU, d.ContextSize, d.Threads
	return a.SaveSettings(s)
}

func (a *App) ChooseModelsDir() (string, error) {
	dir, err := a.openDirectory("Escolha a pasta de modelos", a.settings.Get().ModelsDir)
	if err != nil || dir == "" {
		return "", err
	}
	s := a.settings.Get()
	s.ModelsDir = dir
	_, err = a.SaveSettings(s)
	return dir, err
}

func openPath(path string) error {
	switch goruntime.GOOS {
	case "darwin":
		return exec.Command("open", path).Start()
	case "windows":
		return exec.Command("explorer", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

func (a *App) OpenModelsFolder() error { return openPath(a.settings.Get().ModelsDir) }
func (a *App) OpenDataFolder() error   { return openPath(paths.DataDir()) }

// --------------------------------------------------------------- catalog

func (a *App) ctxSize() int { return a.settings.Get().ContextSize }

func (a *App) GetCategories() []catalog.Category { return catalog.Categories }

func (a *App) GetRecommended(category string) []catalog.View {
	return a.catalog.Recommended(category, a.fitHardware(), a.ctxSize())
}

func (a *App) GetTopPicks() map[string]catalog.View {
	return a.catalog.TopPicks(a.fitHardware(), a.ctxSize())
}

func (a *App) GetInstalled() []catalog.View {
	return a.catalog.Installed(a.fitHardware(), a.ctxSize())
}

func (a *App) GetDownloads() []downloads.Status { return a.downloads.Active() }

func (a *App) DownloadModel(id, quant string) error {
	m, files, err := a.catalog.DownloadPlan(id, quant, a.fitHardware(), a.ctxSize())
	if err != nil {
		return err
	}
	var need int64
	plan := make([]downloads.File, 0, len(files))
	for _, f := range files {
		plan = append(plan, downloads.File{URL: f.URL, Dest: a.catalog.Dest(f), Size: f.Size})
		need += f.Size
	}
	if free := hardware.DiskFreeGB(a.settings.Get().ModelsDir); free > 0 && free*hardware.GiB < float64(need)*1.05 {
		return fmt.Errorf("espaço insuficiente: são necessários %.1f GB e há %.1f GB livres",
			float64(need)/hardware.GiB, free)
	}
	return a.downloads.Start(modelKeyPrefix+id, m.Name, plan)
}

func (a *App) CancelDownload(key string) { a.downloads.Cancel(key) }

func (a *App) RemoveModel(id string) error {
	if a.runtime.Status().ModelID == id {
		a.runtime.Stop()
	}
	if a.embedder.Status().ModelID == id {
		a.embedder.Stop()
	}
	if err := a.catalog.Remove(id); err != nil {
		return err
	}
	_ = a.updateSettings(func(s *settings.Settings) {
		for _, field := range []*string{&s.LastModelID, &s.VoiceModelID, &s.EmbedModelID, &s.ImageModelID} {
			if *field == id {
				*field = ""
			}
		}
	})
	a.emit("catalog:changed")
	return nil
}

func (a *App) downloadDone(key string, err error) {
	errMsg := ""
	if err != nil && !errors.Is(err, downloads.ErrCanceled) {
		errMsg = err.Error()
	}
	if strings.HasPrefix(key, "tool:") {
		a.mu.Lock()
		dest := a.pendingTools[key]
		delete(a.pendingTools, key)
		a.mu.Unlock()
		if err == nil && dest != "" {
			if ierr := a.toolInstalled(key, dest); ierr != nil {
				errMsg = ierr.Error()
			}
		}
		a.emit("tools:changed")
		go a.detectToolGPU(false)
	} else if id, ok := strings.CutPrefix(key, modelKeyPrefix); ok && err == nil {
		a.modelDownloaded(id)
		go a.detectToolGPU(false) // a Whisper model enables probing whisperfile
	}
	a.emit("downloads:done", map[string]string{"key": key, "error": errMsg})
	a.emit("catalog:changed")
}

// modelDownloaded makes a freshly downloaded model the default of its kind
// when none is chosen yet.
func (a *App) modelDownloaded(id string) {
	m := a.catalog.Find(id)
	if m == nil {
		return
	}
	_ = a.updateSettings(func(s *settings.Settings) {
		switch {
		case m.IsChat() && s.LastModelID == "":
			s.LastModelID = id
		case m.HasCategory("voice") && s.VoiceModelID == "":
			s.VoiceModelID = id
		case m.HasCategory("tts") && s.TTSVoiceID == "":
			s.TTSVoiceID = id
		case m.HasCategory("embedding") && s.EmbedModelID == "":
			s.EmbedModelID = id
		case m.HasCategory("image") && s.ImageModelID == "":
			s.ImageModelID = id
		}
	})
}

// installedFor returns installed models of a category, the preferred one first.
func (a *App) installedFor(category, preferred string) []catalog.View {
	list := a.catalog.InstalledFor(category, a.fitHardware(), a.ctxSize())
	for i, v := range list {
		if v.ID == preferred && i > 0 {
			list[0], list[i] = list[i], list[0]
		}
	}
	return list
}

func (a *App) GetInstalledFor(category string) []catalog.View {
	s := a.settings.Get()
	preferred := map[string]string{"voice": s.VoiceModelID, "embedding": s.EmbedModelID, "image": s.ImageModelID}[category]
	return a.installedFor(category, preferred)
}

// --------------------------------------------------------------- runtime

func (a *App) GetRuntimeStatus() runtime.Status  { return a.runtime.Status() }
func (a *App) GetEmbedderStatus() runtime.Status { return a.embedder.Status() }

func (a *App) DetectLlamafile() runtime.Status {
	a.embedder.Detect()
	return a.runtime.Detect()
}

func (a *App) ChooseLlamafileBinary() (runtime.Status, error) {
	path, err := a.openFile("Selecione o executável do llamafile")
	if err != nil || path == "" {
		return a.runtime.Status(), err
	}
	if err := a.updateSettings(func(s *settings.Settings) { s.LlamafilePath = path }); err != nil {
		return a.runtime.Status(), err
	}
	_ = a.runtime.Installed(path)
	a.embedder.Detect()
	a.emit("catalog:changed")
	return a.runtime.Status(), nil
}

func (a *App) StartModel(id string) error {
	if err := a.runtime.Start(id); err != nil {
		return err
	}
	_ = a.updateSettings(func(s *settings.Settings) { s.LastModelID = id })
	return nil
}

func (a *App) StopServer()          { a.runtime.Stop() }
func (a *App) StopEmbedder()        { a.embedder.Stop() }
func (a *App) RestartServer() error { return a.runtime.Restart() }
func (a *App) GetLogs() []string    { return a.runtime.Logs() }

// toolInstalled finishes a tool download (chmod, select, re-detect).
func (a *App) toolInstalled(key, dest string) error {
	if key == tools.DownloadKey(piperTool) {
		return tts.Install(dest)
	}
	if key == tools.DownloadKey(tools.SDCpp) {
		return tools.InstallSDCpp(dest, sdcppVersion(dest))
	}
	if key == tools.DownloadKey(tools.Llamafile) {
		if err := a.updateSettings(func(s *settings.Settings) { s.LlamafilePath = dest }); err != nil {
			return err
		}
		if err := a.runtime.Installed(dest); err != nil {
			return err
		}
		a.embedder.Detect()
		return nil
	}
	return chmodExec(dest)
}
