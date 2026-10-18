package catalog

import (
	"math"
	"strings"

	"github.com/devalexandre/llamafile-ui/internal/hardware"
	"github.com/devalexandre/llamafile-ui/internal/i18n"
)

// Fit describes how well one quant of a model runs on this machine.
//
// Inspired by llmfit (https://github.com/AlexsJones/llmfit), specialised for llamafile/GGUF:
//
//	required = weights + extras + KV cache(ctx) + runtime overhead
//	fit      = required / memory pool  (≤60% perfect, ≤85% good, ≤98% marginal)
//	speed    = LLM: bandwidth / bytes read per token; Whisper: realtime factor;
//	           diffusion: seconds per image (calibrated on CPU runs)
//	score    = weighted(quality, speed, fit, context) per use case
type Fit struct {
	Quant           string  `json:"quant"`
	File            string  `json:"file"`
	DownloadBytes   int64   `json:"downloadBytes"`
	RequiredGB      float64 `json:"requiredGb"`
	Mode            string  `json:"mode"` // GPU, GPU+CPU, CPU, Metal
	Utilization     float64 `json:"utilization"`
	Level           string  `json:"level"` // perfect, good, marginal, tight
	Label           string  `json:"label"`
	TPS             float64 `json:"tps"`             // LLMs
	RealtimeFactor  float64 `json:"realtimeFactor"`  // Whisper: seconds of work per second of audio
	SecondsPerImage float64 `json:"secondsPerImage"` // diffusion
	SpeedLabel      string  `json:"speedLabel"`
	QualityScore    float64 `json:"qualityScore"`
	SpeedScore      float64 `json:"speedScore"`
	FitScore        float64 `json:"fitScore"`
	ContextScore    float64 `json:"contextScore"`
	Score           float64 `json:"score"`
	Category        string  `json:"category"`
	Measured        bool    `json:"measured"`   // speed comes from runs of this model on this machine
	Calibrated      bool    `json:"calibrated"` // estimate adjusted by runs of other models
	Samples         int     `json:"samples"`
	speed           int     // index in speedNames (language independent)
}

// Calibration adjusts a speed estimate with what was measured on this machine:
// Ratio = measured / estimated (tok/s for LLMs; seconds for images and audio).
type Calibration struct {
	Ratio    float64
	Measured bool // ratio comes from this exact model and quant
	Samples  int
}

// Calibrator looks up the calibration for a model run (kind: llm, image, voice).
type Calibrator func(modelID, quant, kind, mode string) (Calibration, bool)

// KindOf groups models by how their speed is measured.
func KindOf(m *Model, category string) string {
	switch {
	case m.Runtime == RuntimeWhisper:
		return "voice"
	case m.Runtime == RuntimeDiffusion && m.Video != nil:
		return "video"
	case m.Runtime == RuntimeDiffusion:
		return "image"
	case m.Runtime == RuntimePiper:
		return "tts"
	case category == "embedding" || m.HasCategory("embedding"):
		return "embedding"
	}
	return "llm"
}

type weights struct {
	quality, speed, fit, context float64
	targetTPS                    float64 // tok/s that feels "fast enough"
	targetContext                int     // tokens the use case usually needs
}

func weightsFor(category string) weights {
	switch category {
	case "code":
		return weights{0.50, 0.25, 0.15, 0.10, 25, 16384}
	case "reasoning":
		return weights{0.55, 0.20, 0.15, 0.10, 20, 16384}
	case "vision":
		return weights{0.50, 0.20, 0.20, 0.10, 15, 8192}
	case "image":
		return weights{0.45, 0.40, 0.15, 0, 0, 0}
	case "voice", "video":
		return weights{0.45, 0.40, 0.15, 0, 0, 0}
	case "tts":
		return weights{0.60, 0.25, 0.15, 0, 0, 0}
	case "embedding":
		return weights{0.55, 0.15, 0.30, 0, 0, 0}
	default: // chat
		return weights{0.40, 0.30, 0.20, 0.10, 25, 8192}
	}
}

func quantPenalty(quant string) float64 {
	q := strings.ToUpper(quant)
	switch {
	case strings.HasPrefix(q, "Q2"), strings.HasPrefix(q, "Q3"):
		return 8
	case strings.HasPrefix(q, "Q4"):
		return 4
	case strings.HasPrefix(q, "Q5"):
		return 2
	}
	return 0 // Q6/Q8/F16/BF16/MXFP4
}

func levelFor(u float64) string {
	switch {
	case u <= 0.60:
		return "perfect"
	case u <= 0.85:
		return "good"
	case u <= 0.98:
		return "marginal"
	}
	return "tight"
}

func LevelLabel(level string) string {
	switch level {
	case "perfect":
		return i18n.T("Roda perfeitamente")
	case "good":
		return i18n.T("Roda bem")
	case "marginal":
		return i18n.T("No limite")
	}
	return i18n.T("Não cabe")
}

func fitScoreFor(u float64) float64 {
	switch {
	case u > 0.98:
		return 0
	case u <= 0.5:
		return 90 // room to spare: fine, but a bigger model could fit too
	case u <= 0.8:
		return 100
	}
	return 100 - (u-0.8)/0.18*50
}

var speedNames = [4]string{i18n.N("Muito rápido"), i18n.N("Rápido"), i18n.N("Usável"), i18n.N("Lento")}

// speedSlow is the index of "Lento" in speedNames.
const speedSlow = 3

// speedLevel returns the index in speedNames for v.
func speedLevel(v float64, thresholds [3]float64, lowerIsFaster bool) int {
	for i, t := range thresholds {
		if (lowerIsFaster && v < t) || (!lowerIsFaster && v >= t) {
			return i
		}
	}
	return speedSlow
}

// setSpeedLevel stores the speed level and its translated label.
func (f *Fit) setSpeedLevel(i int) {
	f.speed = i
	f.SpeedLabel = i18n.T(speedNames[i])
}

func quality(m *Model, category, quant string) float64 {
	q, ok := m.Quality[category]
	if !ok {
		best := 0.0
		for _, v := range m.Quality {
			best = max(best, v)
		}
		q = best * 0.85
	}
	return max(0, q-quantPenalty(quant))
}

// Evaluate scores one quant file of a model for a category (estimates only).
func Evaluate(m *Model, f File, category string, hw hardware.Info, contextSize int) Fit {
	return EvaluateWith(m, f, category, hw, contextSize, nil)
}

// EvaluateWith scores a model, refining the speed estimate with cal when given.
func EvaluateWith(m *Model, f File, category string, hw hardware.Info, contextSize int, cal Calibrator) Fit {
	var fit Fit
	switch m.Runtime {
	case RuntimeWhisper:
		fit = evaluateWhisper(m, f, hw)
	case RuntimeDiffusion:
		fit = evaluateDiffusion(m, f, hw)
	case RuntimePiper:
		fit = evaluateTTS(f, hw)
	default:
		fit = evaluateLLM(m, f, category, hw, contextSize)
	}
	fit.Quant, fit.File, fit.Category = f.Quant, f.File, category
	fit.DownloadBytes = f.Size + m.extrasSize()
	fit.Label = LevelLabel(fit.Level)
	fit.QualityScore = quality(m, category, f.Quant)
	if kind := KindOf(m, category); cal != nil && kind != "embedding" {
		if c, ok := cal(m.ID, f.Quant, kind, fit.Mode); ok && c.Ratio > 0 {
			switch kind {
			case "llm":
				fit.TPS *= c.Ratio
			case "voice", "tts":
				fit.RealtimeFactor *= c.Ratio
			case "image", "video":
				fit.SecondsPerImage *= c.Ratio
			}
			fit.Measured, fit.Calibrated, fit.Samples = c.Measured, !c.Measured, c.Samples
			setSpeed(&fit, kind, category)
		}
	}
	fit.FitScore = fitScoreFor(fit.Utilization)

	w := weightsFor(category)
	fit.Score = w.quality*fit.QualityScore + w.speed*fit.SpeedScore + w.fit*fit.FitScore + w.context*fit.ContextScore
	if fit.speed == speedSlow && category != "embedding" {
		// Too slow on this machine (e.g. transcription slower than the audio):
		// recommend a faster model instead, even if a bit less accurate.
		fit.Score -= 20
	}
	if fit.Level == "tight" {
		fit.Score = fit.Score*0.2 - 100 // always below anything that runs
	}
	return fit
}

func evaluateLLM(m *Model, f File, category string, hw hardware.Info, contextSize int) Fit {
	const GiB = hardware.GiB
	weightsGB := float64(f.Size) / GiB
	extrasGB := float64(m.extrasSize()) / GiB
	ctx := 0
	if m.Context > 0 {
		ctx = min(contextSize, m.Context)
	}
	kvGB := m.KVMBPer1K * float64(ctx) / 1024 / 1024
	required := weightsGB*1.05 + extrasGB + kvGB + 0.35

	active := m.ActiveParamsB
	if active <= 0 {
		active = m.ParamsB
	}
	// Bytes touched per generated token: MoE only reads the active experts.
	perToken := weightsGB * active / m.ParamsB

	ram, vram := hw.UsableRAMGB, hw.UsableVRAMGB
	if hw.LLMCPUOnly && !hw.UnifiedMemory {
		vram = 0
	}
	gpuBW, cpuBW := hw.GPUBandwidthGBps, hw.CPUBandwidthGBps

	var fit Fit
	capGood := false
	var pool float64
	switch {
	case hw.UnifiedMemory:
		fit.Mode, pool, fit.TPS = "Metal", ram, gpuBW/perToken*0.55
	case vram > 0 && required <= vram:
		fit.Mode, pool, fit.TPS = "GPU", vram, gpuBW/perToken*0.55
	case vram > 0 && required <= vram+ram:
		// Partial offload: some layers on the GPU, the rest on the CPU.
		onGPU := min(max(vram/required, 0), 1)
		secondsPerToken := perToken*onGPU/gpuBW + perToken*(1-onGPU)/cpuBW
		fit.Mode, pool, fit.TPS = "GPU+CPU", vram+ram, 0.55/secondsPerToken
		capGood = true
	default:
		fit.Mode, pool, fit.TPS = "CPU", ram, cpuBW/perToken*0.5
		capGood = true
	}
	fit.RequiredGB = required
	fit.Utilization = utilization(required, pool)
	fit.Level = levelFor(fit.Utilization)
	if capGood && fit.Level == "perfect" {
		fit.Level = "good"
	}

	w := weightsFor(category)
	if category == "embedding" {
		fit.SpeedScore = min(max(100-m.ParamsB*40, 10), 100)
		fit.setSpeedLevel(speedLevel(m.ParamsB, [3]float64{0.3, 1, 3}, true))
		fit.TPS = 0
	} else {
		setSpeed(&fit, "llm", category)
	}
	fit.ContextScore = 100
	if w.targetContext > 0 {
		fit.ContextScore = min(1, float64(m.Context)/float64(w.targetContext)) * 100
	}
	return fit
}

// setSpeed derives the speed score and label from the speed metric of kind.
func setSpeed(fit *Fit, kind, category string) {
	switch kind {
	case "voice":
		fit.SpeedScore = 100 * math.Exp(-fit.RealtimeFactor)
		fit.setSpeedLevel(speedLevel(fit.RealtimeFactor, [3]float64{0.25, 0.6, 1}, true))
	case "tts":
		fit.SpeedScore = 100 * math.Exp(-fit.RealtimeFactor/0.5)
		fit.setSpeedLevel(speedLevel(fit.RealtimeFactor, [3]float64{0.2, 0.5, 1}, true))
	case "video":
		fit.SpeedScore = 100 * math.Exp(-fit.SecondsPerImage/300)
		fit.setSpeedLevel(speedLevel(fit.SecondsPerImage, [3]float64{60, 180, 600}, true))
	case "image":
		fit.SpeedScore = 100 * math.Exp(-fit.SecondsPerImage/30)
		fit.setSpeedLevel(speedLevel(fit.SecondsPerImage, [3]float64{10, 30, 90}, true))
	default:
		fit.SpeedScore = 100 * (1 - math.Exp(-fit.TPS/weightsFor(category).targetTPS))
		fit.setSpeedLevel(speedLevel(fit.TPS, [3]float64{40, 20, 8}, false))
	}
}

func utilization(required, pool float64) float64 {
	if pool <= 0 {
		return 99
	}
	return required / pool
}

// cpuFactor scales timings calibrated on a 45 GB/s desktop CPU (Ryzen 5 1600, DDR4).
func cpuFactor(hw hardware.Info) (factor float64, mode string) {
	if hw.UnifiedMemory {
		return 45 / hw.GPUBandwidthGBps / 1.5, "Metal"
	}
	bw := hw.CPUBandwidthGBps
	if bw <= 0 {
		bw = 45
	}
	return 45 / bw, "CPU"
}

// toolFactor is cpuFactor for whisperfile/diffusionfile, which only use the
// GPU when the installed build ships a GPU backend (detected from its logs).
func toolFactor(hw hardware.Info, gpu bool, requiredGB float64) (factor float64, mode string, pool float64) {
	if gpu && !hw.UnifiedMemory && hw.GPUBandwidthGBps > 0 && requiredGB <= hw.UsableVRAMGB {
		return 45 / hw.GPUBandwidthGBps, "GPU", hw.UsableVRAMGB
	}
	factor, mode = cpuFactor(hw)
	return factor, mode, hw.UsableRAMGB
}

// whisperfile runs on the CPU (Metal on Apple Silicon) unless its build has a GPU backend.
func evaluateWhisper(m *Model, f File, hw hardware.Info) Fit {
	weightsGB := float64(f.Size) / hardware.GiB
	required := weightsGB*1.3 + 0.2
	factor, mode, pool := toolFactor(hw, hw.WhisperGPU, required)
	fit := Fit{Mode: mode, RequiredGB: required}
	fit.Utilization = utilization(required, pool)
	fit.Level = levelFor(fit.Utilization)
	// Measured with greedy decoding (the app default) on 69 s of speech:
	// Base on the reference CPU (Ryzen 5 1600) 0.87× the audio length;
	// Large v3 Turbo on an RTX 2060 (Vulkan) 0.047×.
	if mode == "GPU" {
		fit.RealtimeFactor = 0.02 + m.ParamsB*0.45*factor
	} else {
		fit.RealtimeFactor = m.ParamsB * 11.8 * factor
	}
	setSpeed(&fit, "voice", "voice")
	fit.ContextScore = 100
	return fit
}

// EstimateImageSeconds predicts the wall time to generate one image.
// Calibrated on SD-Turbo 512²: 10 s/step on the reference CPU, TAESD decode 3 s,
// full VAE decode 48 s.
func EstimateImageSeconds(m *Model, f File, hw hardware.Info, width, height, steps int, fastDecode bool) float64 {
	spec := m.Image
	if spec == nil {
		return 0
	}
	totalGB := float64(f.Size+m.extrasSize()) / hardware.GiB
	factor, _, _ := toolFactor(hw, hw.DiffusionGPU, totalGB*1.1+1.8)
	px := float64(width*height) / (512 * 512)
	perStep := spec.DiffusionParamsB * 11.6 * px * factor
	decode := 48 * px * max(spec.VAECost, 1) * factor
	if fastDecode {
		decode = 3.2 * px * factor
	}
	encode := 0.5
	for _, c := range m.Components {
		if c.Role == "llm" || c.Role == "t5xxl" {
			encode += float64(c.Size) / hardware.GiB * 2 * factor
		}
	}
	load := totalGB / 1.5
	return float64(steps)*perStep + decode + encode + load
}

// diffusionfile runs on the CPU (Metal on Apple Silicon) unless its build has a GPU backend.
func evaluateDiffusion(m *Model, f File, hw hardware.Info) Fit {
	if m.Video != nil {
		return evaluateVideo(m, f, hw)
	}
	spec := m.Image
	totalGB := float64(f.Size+m.extrasSize()) / hardware.GiB
	px := 1.0
	steps := 20
	if spec != nil {
		px = float64(spec.Width*spec.Height) / (512 * 512)
		steps = spec.Steps
	}
	required := totalGB*1.1 + 0.6*px + 1.2*px // weights + UNet/DiT buffers + VAE buffers
	_, mode, pool := toolFactor(hw, hw.DiffusionGPU, required)
	fit := Fit{Mode: mode, RequiredGB: required}
	fit.Utilization = utilization(required, pool)
	fit.Level = levelFor(fit.Utilization)
	if spec != nil {
		fit.SecondsPerImage = EstimateImageSeconds(m, f, hw, spec.Width, spec.Height, steps, hasRole(m, "taesd"))
	}
	setSpeed(&fit, "image", "image")
	fit.ContextScore = 100
	return fit
}

func hasRole(m *Model, role string) bool {
	for _, c := range m.Components {
		if c.Role == role {
			return true
		}
	}
	return false
}

// EvaluateBest picks the quant with the best score that fits, falling back to
// the smallest file when nothing fits.
func EvaluateBest(m *Model, category string, hw hardware.Info, contextSize int) Fit {
	return EvaluateBestWith(m, category, hw, contextSize, nil)
}

// EvaluateBestWith is EvaluateBest with measured speeds applied.
func EvaluateBestWith(m *Model, category string, hw hardware.Info, contextSize int, cal Calibrator) Fit {
	var best, smallest *Fit
	var smallestSize int64 = math.MaxInt64
	for _, f := range m.Files {
		r := EvaluateWith(m, f, category, hw, contextSize, cal)
		if r.Level != "tight" && (best == nil || r.Score > best.Score) {
			best = &r
		}
		if f.Size < smallestSize {
			smallestSize = f.Size
			smallest = &r
		}
	}
	if best != nil {
		return *best
	}
	if smallest != nil {
		return *smallest
	}
	return Fit{Level: "tight", Label: LevelLabel("tight"), Category: category}
}

// evaluateTTS estimates a Piper voice (ONNX Runtime on the CPU).
// Calibrated: a "medium" voice (63 MB) ≈ 0.15× realtime on the reference CPU.
func evaluateTTS(f File, hw hardware.Info) Fit {
	sizeGB := float64(f.Size) / hardware.GiB
	required := sizeGB*3 + 0.15
	factor, mode := cpuFactor(hw)
	if mode == "Metal" {
		mode = "CPU" // ONNX Runtime does not use the Apple GPU here
	}
	fit := Fit{Mode: mode, RequiredGB: required, RealtimeFactor: sizeGB * 2.6 * factor}
	fit.Utilization = utilization(required, hw.UsableRAMGB)
	fit.Level = levelFor(fit.Utilization)
	setSpeed(&fit, "tts", "tts")
	fit.ContextScore = 100
	return fit
}

// videoGPUGB is what must fit in VRAM for a video model: the diffusion model
// and the VAE. stable-diffusion.cpp keeps the big text encoder (umt5) in RAM
// when the GPU is short of memory, and it only runs once per video.
func videoGPUGB(m *Model, f File) float64 {
	gb := float64(f.Size) / hardware.GiB
	for _, c := range m.Components {
		if c.Role == "vae" {
			gb += float64(c.Size) / hardware.GiB
		}
	}
	return gb*1.15 + 1
}

// evaluateVideo estimates a Wan video model at its default size and length.
func evaluateVideo(m *Model, f File, hw hardware.Info) Fit {
	spec := m.Image
	gpuGB := videoGPUGB(m, f)
	_, mode, _ := toolFactor(hw, hw.DiffusionGPU, gpuGB)
	totalGB := float64(f.Size+m.extrasSize())/hardware.GiB*1.1 + 1.5
	fit := Fit{Mode: mode, RequiredGB: totalGB}
	if mode == "GPU" {
		// VRAM holds the diffusion model; the text encoder goes to RAM.
		fit.Utilization = max(utilization(gpuGB, hw.UsableVRAMGB), utilization(totalGB-gpuGB, hw.UsableRAMGB))
	} else {
		fit.Utilization = utilization(totalGB, hw.UsableRAMGB)
	}
	fit.Level = levelFor(fit.Utilization)
	if spec != nil && m.Video != nil {
		fit.SecondsPerImage = EstimateVideoSeconds(m, f, hw, spec.Width, spec.Height, m.Video.Frames, spec.Steps)
	}
	setSpeed(&fit, "video", "video")
	fit.ContextScore = 100
	return fit
}

// EstimateVideoSeconds predicts the wall time of one video. The Wan VAE
// compresses 4 frames into one latent frame; cost grows with pixels × latent
// frames. Calibrated afterwards by real runs (bench).
func EstimateVideoSeconds(m *Model, f File, hw hardware.Info, width, height, frames, steps int) float64 {
	spec := m.Image
	if spec == nil {
		return 0
	}
	factor, _, _ := toolFactor(hw, hw.DiffusionGPU, videoGPUGB(m, f))
	px := float64(width*height) / (512 * 512)
	latentFrames := float64((max(frames, 1)-1)/4 + 1)
	perStep := spec.DiffusionParamsB * 11.6 * px * latentFrames * 0.6 * factor
	decode := 15 * px * latentFrames * max(spec.VAECost, 1) * factor
	encode := 1.0
	for _, c := range m.Components {
		if c.Role == "t5xxl" {
			encode += float64(c.Size) / hardware.GiB * 6 // umt5 weights stay in RAM
		}
	}
	load := float64(f.Size+m.extrasSize()) / hardware.GiB / 1.5
	return float64(steps)*perStep + decode + encode + load
}
