package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/devalexandre/llamafile-ui/internal/bench"
	"github.com/devalexandre/llamafile-ui/internal/catalog"
	"github.com/devalexandre/llamafile-ui/internal/chat"
	"github.com/devalexandre/llamafile-ui/internal/imagegen"
	"github.com/devalexandre/llamafile-ui/internal/tools"
)

// Measurements shorter than these are dominated by start-up costs.
const (
	minChatTokens   = 16
	minAudioSeconds = 10
)

// estimate evaluates an installed model without calibration: the reference
// that measurements are compared against.
func (a *App) estimate(id, category string) (*catalog.Model, catalog.File, catalog.Fit, bool) {
	m := a.catalog.Find(id)
	if m == nil {
		return nil, catalog.File{}, catalog.Fit{}, false
	}
	f, ok := a.catalog.InstalledFile(m)
	if !ok {
		return nil, catalog.File{}, catalog.Fit{}, false
	}
	return m, f, catalog.Evaluate(m, f, category, a.fitHardware(), a.ctxSize()), true
}

func (a *App) observe(m *catalog.Model, f catalog.File, kind, mode string, measured, estimated, promptTPS float64) {
	r := a.bench.Observe(m.ID, f.Quant, kind, mode, measured, estimated, promptTPS)
	if r.Samples > 0 {
		a.emit("bench:updated", r)
		a.emit("catalog:changed")
	}
}

// observeChat learns from every finished answer (llama-server reports the
// generation speed in "timings").
func (a *App) observeChat(msg chat.Message) {
	if msg.Error != "" || msg.Tokens < minChatTokens || msg.TPS <= 0 {
		return
	}
	if m, f, fit, ok := a.estimate(msg.ModelID, "chat"); ok && fit.TPS > 0 {
		a.observe(m, f, "llm", fit.Mode, msg.TPS, fit.TPS, 0)
	}
}

func (a *App) observeImage(img imagegen.Image) {
	if img.DurationMs <= 0 || img.Reference != "" { // img2img runs a different number of steps
		return
	}
	m, f, fit, ok := a.estimate(img.ModelID, "image")
	if !ok {
		return
	}
	if img.Kind == "video" {
		est := catalog.EstimateVideoSeconds(m, f, a.fitHardware(), img.Width, img.Height, img.Frames, img.Steps)
		a.observe(m, f, "video", fit.Mode, float64(img.DurationMs)/1000, est, 0)
		return
	}
	_, hasTAESD := a.catalog.ComponentPaths(img.ModelID)["taesd"]
	est := catalog.EstimateImageSeconds(m, f, a.fitHardware(), img.Width, img.Height, img.Steps, a.settings.Get().FastDecode && hasTAESD)
	a.observe(m, f, "image", fit.Mode, float64(img.DurationMs)/1000, est, 0)
}

func (a *App) observeTranscription(modelID string, audioSeconds float64, took time.Duration) {
	if audioSeconds < minAudioSeconds {
		return
	}
	if m, f, fit, ok := a.estimate(modelID, "voice"); ok && fit.RealtimeFactor > 0 {
		a.observe(m, f, "voice", fit.Mode, took.Seconds()/audioSeconds, fit.RealtimeFactor, 0)
	}
}

// calibratedImageSeconds applies this machine's measurements to an image estimate.
func (a *App) calibratedImageSeconds(m *catalog.Model, f catalog.File, seconds float64) float64 {
	fit := catalog.Evaluate(m, f, "image", a.fitHardware(), a.ctxSize())
	if c, ok := a.bench.Calibrator()(m.ID, f.Quant, catalog.KindOf(m, ""), fit.Mode); ok && c.Ratio > 0 {
		return seconds * c.Ratio
	}
	return seconds
}

// BenchResult is the outcome of an explicit benchmark.
type BenchResult struct {
	ModelID     string  `json:"modelId"`
	TPS         float64 `json:"tps"`
	PromptTPS   float64 `json:"promptTps"`
	EstimateTPS float64 `json:"estimateTps"`
	Tokens      int     `json:"tokens"`
	Mode        string  `json:"mode"`
}

const benchPrompt = "Escreva um texto corrido sobre a história da computação pessoal, dos primeiros microcomputadores aos notebooks atuais."

// BenchmarkModel loads an LLM in the chat server and measures its generation
// speed with a fixed prompt. The result calibrates every estimate.
func (a *App) BenchmarkModel(id string) (BenchResult, error) {
	m, f, fit, ok := a.estimate(id, "chat")
	if !ok {
		return BenchResult{}, errors.New("o modelo não está instalado")
	}
	if m.Runtime == catalog.RuntimeWhisper || m.Runtime == catalog.RuntimeDiffusion || catalog.KindOf(m, "") == "embedding" {
		return BenchResult{}, errors.New("a velocidade deste modelo é medida automaticamente ao usá-lo")
	}
	a.emit("bench:state", map[string]any{"modelId": id, "phase": "loading"})
	defer a.emit("bench:state", map[string]any{"modelId": id, "phase": "done"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := (&backend{a}).EnsureModel(ctx, id); err != nil {
		return BenchResult{}, err
	}
	a.emit("bench:state", map[string]any{"modelId": id, "phase": "running"})
	// Warm-up so the measured run does not include first-token costs.
	if _, err := a.completionTimings(ctx, 8); err != nil {
		return BenchResult{}, err
	}
	t, err := a.completionTimings(ctx, 160)
	if err != nil {
		return BenchResult{}, err
	}
	if t.PredictedPerSecond <= 0 {
		return BenchResult{}, errors.New("o servidor não informou a velocidade")
	}
	a.observe(m, f, "llm", fit.Mode, t.PredictedPerSecond, fit.TPS, t.PromptPerSecond)
	return BenchResult{ModelID: id, TPS: t.PredictedPerSecond, PromptTPS: t.PromptPerSecond,
		EstimateTPS: fit.TPS, Tokens: t.PredictedN, Mode: fit.Mode}, nil
}

type timings struct {
	PromptPerSecond    float64 `json:"prompt_per_second"`
	PredictedPerSecond float64 `json:"predicted_per_second"`
	PredictedN         int     `json:"predicted_n"`
}

func (a *App) completionTimings(ctx context.Context, tokens int) (timings, error) {
	body, _ := json.Marshal(map[string]any{
		"prompt": benchPrompt, "n_predict": tokens, "ignore_eos": true,
		"cache_prompt": false, "temperature": 0.7, "stream": false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.runtime.Status().BaseURL+"/completion", bytes.NewReader(body))
	if err != nil {
		return timings{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return timings{}, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return timings{}, fmt.Errorf("servidor: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Timings timings `json:"timings"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return timings{}, err
	}
	return out.Timings, nil
}

func (a *App) GetBenchmarks() []bench.Record { return a.bench.Records() }

func (a *App) ResetBenchmarks() {
	a.bench.Reset()
	a.emit("catalog:changed")
}

// ------------------------------------------------------ tool capabilities

// detectToolGPU probes whisperfile and diffusionfile builds not checked yet
// (new install or new version) for GPU backends.
func (a *App) detectToolGPU(force bool) {
	changed := false
	for _, t := range []tools.Tool{tools.Whisper, tools.WhisperGPU, tools.Diffusion, tools.SDCpp} {
		bin, version := a.nativeOrFind(t)
		if bin == "" {
			continue
		}
		if _, ok := a.bench.Capability(string(t), version); ok && !force {
			continue
		}
		model := ""
		if t == tools.Whisper || t == tools.WhisperGPU {
			if v, err := a.voiceModel(); err == nil {
				model = a.catalog.ModelPath(v.ID)
			}
		}
		kind := t
		switch t {
		case tools.SDCpp:
			kind = tools.Diffusion // same program, same logs
		case tools.WhisperGPU:
			kind = tools.Whisper
		}
		backends, err := tools.ProbeBackends(context.Background(), kind, bin, model)
		if err != nil {
			continue
		}
		a.bench.SetCapability(string(t), version, backends)
		changed = true
	}
	if changed {
		a.emit("tools:changed")
		a.emit("catalog:changed")
	}
}

// toolGPU reports whether the installed build of a tool has a GPU backend.
func (a *App) toolGPU(t tools.Tool) (bench.Capability, bool) {
	bin, version := a.nativeOrFind(t)
	if bin == "" {
		return bench.Capability{}, false
	}
	return a.bench.Capability(string(t), version)
}
