package bench

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/devalexandre/llamafile-ui/internal/catalog"
	"github.com/devalexandre/llamafile-ui/internal/hardware"
)

var hw = hardware.Info{CPUName: "Ryzen", GPUName: "RTX 2060", TotalRAMGB: 31.3, UsableRAMGB: 25,
	VRAMGB: 6, UsableVRAMGB: 5.5, GPUBandwidthGBps: 336, CPUBandwidthGBps: 45}

func TestCalibrationMeasuredAndBorrowed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bench.json")
	s := Open(path, hw)
	cal := s.Calibrator()
	if _, ok := cal("qwen3-4b", "Q4_K_M", "llm", "GPU"); ok {
		t.Fatal("no data yet: expected no calibration")
	}

	// Estimated 80 tok/s, measured 56 → ratio 0.7.
	s.Observe("qwen3-4b", "Q4_K_M", "llm", "GPU", 56, 80, 900)
	c, ok := cal("qwen3-4b", "Q4_K_M", "llm", "GPU")
	if !ok || !c.Measured || math.Abs(c.Ratio-0.7) > 1e-9 {
		t.Fatalf("measured calibration: %+v %v", c, ok)
	}
	// Another GPU LLM borrows the median; CPU runs and images do not.
	if c, ok := cal("phi-4-mini", "Q4_K_M", "llm", "GPU"); !ok || c.Measured || math.Abs(c.Ratio-0.7) > 1e-9 {
		t.Fatalf("borrowed calibration: %+v %v", c, ok)
	}
	if _, ok := cal("phi-4-mini", "Q4_K_M", "llm", "CPU"); ok {
		t.Fatal("CPU mode must not borrow GPU measurements")
	}
	if _, ok := cal("sd-turbo", "Q8_0", "image", "GPU"); ok {
		t.Fatal("images must not borrow LLM measurements")
	}

	// Moving average and persistence.
	s.Observe("qwen3-4b", "Q4_K_M", "llm", "GPU", 64, 80, 0) // ratio 0.8
	r, _ := s.Lookup("qwen3-4b", "Q4_K_M", "GPU")
	if r.Samples != 2 || r.Ratio <= 0.7 || r.Ratio >= 0.8 || r.PromptTPS != 900 {
		t.Fatalf("unexpected record %+v", r)
	}
	if r2, ok := Open(path, hw).Lookup("qwen3-4b", "Q4_K_M", "GPU"); !ok || r2.Samples != 2 {
		t.Fatal("record not persisted")
	}
	// Different hardware → measurements discarded.
	other := hw
	other.GPUName = "RTX 4090"
	if _, ok := Open(path, other).Lookup("qwen3-4b", "Q4_K_M", "GPU"); ok {
		t.Fatal("measurements of other hardware must be ignored")
	}
}

func TestCalibrationChangesFit(t *testing.T) {
	c, err := catalog.New(func() string { return t.TempDir() })
	if err != nil {
		t.Fatal(err)
	}
	m := c.Find("qwen3-4b")
	f := m.Files[0]
	est := catalog.Evaluate(m, f, "chat", hw, 8192)

	s := Open(filepath.Join(t.TempDir(), "b.json"), hw)
	s.Observe(m.ID, f.Quant, "llm", est.Mode, est.TPS/2, est.TPS, 0)
	got := catalog.EvaluateWith(m, f, "chat", hw, 8192, s.Calibrator())
	if !got.Measured || math.Abs(got.TPS-est.TPS/2) > 1e-6 || got.SpeedScore >= est.SpeedScore {
		t.Fatalf("calibrated fit %+v vs estimate %+v", got, est)
	}
}

func TestCapabilities(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "c.json"), hw)
	if c := s.SetCapability("whisperfile", "0.10.6", []string{"CPU"}); c.GPU {
		t.Fatal("CPU-only build reported as GPU")
	}
	if c := s.SetCapability("whisperfile", "0.10.7", []string{"CUDA", "CPU"}); !c.GPU {
		t.Fatal("CUDA build not reported as GPU")
	}
	if _, ok := s.Capability("whisperfile", "0.10.6"); ok {
		t.Fatal("capability of an older version must not apply")
	}
}
