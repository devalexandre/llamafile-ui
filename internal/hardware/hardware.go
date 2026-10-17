// Package hardware detects the machine capabilities used by the fit engine:
// RAM, CPU, GPU/VRAM, unified memory (Apple Silicon) and memory bandwidth.
package hardware

import (
	"context"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/llamafile-ui/internal/sysproc"
)

const GiB = 1024 * 1024 * 1024

type Info struct {
	OS               string  `json:"os"`
	Arch             string  `json:"arch"`
	CPUName          string  `json:"cpuName"`
	CPUThreads       int     `json:"cpuThreads"`
	TotalRAMGB       float64 `json:"totalRamGb"`
	AvailableRAMGB   float64 `json:"availableRamGb"`
	GPUName          string  `json:"gpuName"`
	GPUVendor        string  `json:"gpuVendor"` // nvidia, amd, apple, ""
	VRAMGB           float64 `json:"vramGb"`
	UnifiedMemory    bool    `json:"unifiedMemory"`
	Backend          string  `json:"backend"` // cuda, rocm, metal, cpu
	GPUBandwidthGBps float64 `json:"gpuBandwidthGbps"`
	CPUBandwidthGBps float64 `json:"cpuBandwidthGbps"`
	UsableRAMGB      float64 `json:"usableRamGb"`
	UsableVRAMGB     float64 `json:"usableVramGb"`
	Tier             string  `json:"tier"`
	// GPU backends detected in the installed whisperfile / diffusionfile builds.
	WhisperGPU   bool `json:"whisperGpu"`
	DiffusionGPU bool `json:"diffusionGpu"`
}

type bw struct {
	name string
	gbps float64
}

// Approximate memory bandwidth (GB/s). More specific names must come first.
var nvidiaBandwidth = []bw{
	{"5090", 1792}, {"5080", 960}, {"5070 Ti", 896}, {"5070", 672}, {"5060 Ti", 448}, {"5060", 448},
	{"4090", 1008}, {"4080", 717}, {"4070 Ti", 504}, {"4070", 504}, {"4060 Ti", 288}, {"4060", 272},
	{"3090", 936}, {"3080", 760}, {"3070", 448}, {"3060 Ti", 448}, {"3060", 360}, {"3050", 224},
	{"2080 Ti", 616}, {"2080", 448}, {"2070", 448}, {"2060", 336}, {"1660", 192}, {"1650", 128},
	{"1080 Ti", 484}, {"1080", 320}, {"1070", 256}, {"1060", 192},
	{"H100", 3350}, {"A100", 1555}, {"A6000", 768}, {"L40", 864}, {"L4", 300}, {"T4", 320},
}

var appleBandwidth = []bw{
	{"M1 Ultra", 800}, {"M1 Max", 400}, {"M1 Pro", 200}, {"M1", 68},
	{"M2 Ultra", 800}, {"M2 Max", 400}, {"M2 Pro", 200}, {"M2", 100},
	{"M3 Ultra", 819}, {"M3 Max", 400}, {"M3 Pro", 150}, {"M3", 100},
	{"M4 Max", 546}, {"M4 Pro", 273}, {"M4", 120}, {"M5", 153},
}

func lookup(table []bw, name string, fallback float64) float64 {
	lower := strings.ToLower(name)
	for _, e := range table {
		if strings.Contains(lower, strings.ToLower(e.name)) {
			return e.gbps
		}
	}
	return fallback
}

func run(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	sysproc.Prepare(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Detect inspects the machine. It may take up to a few seconds (nvidia-smi).
func Detect() Info {
	info := Info{
		OS:         osName(),
		Arch:       runtime.GOARCH,
		CPUThreads: runtime.NumCPU(),
	}
	info.TotalRAMGB, info.AvailableRAMGB = memory()
	info.CPUName = cpuName()
	if info.CPUName == "" {
		info.CPUName = runtime.GOARCH
	}
	// Dual-channel DDR4 ≈ 40-50 GB/s, DDR5 ≈ 60-80 GB/s. Conservative default.
	info.CPUBandwidthGBps = 45
	if runtime.GOARCH == "arm64" {
		info.CPUBandwidthGBps = 60
	}

	switch {
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		info.UnifiedMemory = true
		info.GPUVendor = "apple"
		info.GPUName = info.CPUName
		info.GPUBandwidthGBps = lookup(appleBandwidth, info.GPUName, 100)
		info.CPUBandwidthGBps = info.GPUBandwidthGBps
	case detectNvidia(&info):
	default:
		detectAMD(&info)
	}

	info.Backend = "cpu"
	switch info.GPUVendor {
	case "apple":
		info.Backend = "metal"
	case "nvidia":
		info.Backend = "cuda"
	case "amd":
		info.Backend = "rocm"
	}

	if info.UnifiedMemory {
		info.UsableRAMGB = info.TotalRAMGB * 0.70 // Metal wires ~70% of unified memory by default
	} else {
		reserve := clamp(info.TotalRAMGB*0.2, 2, 6)
		info.UsableRAMGB = max(0, info.TotalRAMGB-reserve)
		info.UsableVRAMGB = max(0, info.VRAMGB-0.5)
	}
	info.Tier = tier(info)
	return info
}

func detectNvidia(info *Info) bool {
	out := run("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits")
	if out == "" {
		return false
	}
	// Use the largest GPU; multi-GPU split is not modelled.
	for _, line := range strings.Split(out, "\n") {
		cols := strings.Split(line, ",")
		if len(cols) < 2 {
			continue
		}
		mib, _ := strconv.ParseFloat(strings.TrimSpace(cols[1]), 64)
		if vram := mib / 1024; vram > info.VRAMGB {
			info.VRAMGB = vram
			info.GPUName = strings.TrimSpace(cols[0])
		}
	}
	if info.VRAMGB <= 0 {
		return false
	}
	info.GPUVendor = "nvidia"
	info.GPUBandwidthGBps = lookup(nvidiaBandwidth, info.GPUName, 300)
	if strings.Contains(strings.ToLower(info.GPUName), "laptop") {
		info.GPUBandwidthGBps *= 0.8
	}
	return true
}

func tier(info Info) string {
	best := max(info.UsableVRAMGB, info.UsableRAMGB*0.5)
	if info.UnifiedMemory {
		best = info.UsableRAMGB
	}
	switch {
	case best >= 20:
		return "Estação de trabalho"
	case best >= 10:
		return "Alto desempenho"
	case best >= 5:
		return "Intermediária"
	default:
		return "Básica"
	}
}

func clamp(v, lo, hi float64) float64 { return min(max(v, lo), hi) }
