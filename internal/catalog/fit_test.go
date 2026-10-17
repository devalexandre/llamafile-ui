package catalog

import (
	"testing"

	"github.com/devalexandre/llamafile-ui/internal/hardware"
)

func TestFitLevels(t *testing.T) {
	c, err := New(func() string { return t.TempDir() })
	if err != nil {
		t.Fatal(err)
	}
	laptop := hardware.Info{TotalRAMGB: 8, UsableRAMGB: 6.4, CPUBandwidthGBps: 45}
	gpu := hardware.Info{TotalRAMGB: 32, UsableRAMGB: 26, VRAMGB: 6, UsableVRAMGB: 5.5,
		GPUBandwidthGBps: 336, CPUBandwidthGBps: 45, GPUVendor: "nvidia"}

	tiny := Evaluate(c.Find("qwen2.5-0.5b"), c.Find("qwen2.5-0.5b").Files[0], "chat", laptop, 8192)
	if tiny.Level == "tight" || tiny.Mode != "CPU" {
		t.Fatalf("0.5B should fit on an 8 GB laptop on CPU, got %+v", tiny)
	}
	big := EvaluateBest(c.Find("gpt-oss-20b"), "chat", laptop, 8192)
	if big.Level != "tight" {
		t.Fatalf("20B should not fit on 8 GB, got %s", big.Level)
	}
	onGPU := EvaluateBest(c.Find("qwen3-4b"), "chat", gpu, 8192)
	if onGPU.Mode != "GPU" {
		t.Fatalf("qwen3-4b should run fully on a 6 GB GPU, got %s (%.1f GB)", onGPU.Mode, onGPU.RequiredGB)
	}
	for _, v := range c.Recommended("chat", laptop, 8192) {
		if v.Fit.Level == "tight" {
			continue
		}
		if v.Fit.Score < 0 {
			t.Fatalf("runnable model with negative score: %s", v.ID)
		}
	}
}

func TestRecommendOnThisMachine(t *testing.T) {
	c, _ := New(func() string { return t.TempDir() })
	hw := hardware.Detect()
	t.Logf("%s | %s | RAM %.1f GB (usable %.1f) | GPU %q %.1f GB @ %.0f GB/s | %s",
		hw.OS, hw.CPUName, hw.TotalRAMGB, hw.UsableRAMGB, hw.GPUName, hw.VRAMGB, hw.GPUBandwidthGBps, hw.Tier)
	for _, cat := range Categories {
		for i, v := range c.Recommended(cat.ID, hw, 8192) {
			if i == 3 {
				break
			}
			t.Logf("%-10s %-26s %-7s %-8s %5.1f GB %6.1f tok/s %5.1fs/img %5.1fxRT score %5.1f %s",
				cat.ID, v.Name, v.Fit.Quant, v.Fit.Mode, v.Fit.RequiredGB, v.Fit.TPS, v.Fit.SecondsPerImage, v.Fit.RealtimeFactor, v.Fit.Score, v.Fit.Label)
		}
	}
}
