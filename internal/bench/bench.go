// Package bench stores speed measurements taken on this machine (chat
// generations, image generations, transcriptions and explicit benchmarks)
// and turns them into calibration ratios for the fit engine.
//
// A ratio is measured / estimated: tokens per second for LLMs, seconds for
// images and audio. Models never measured borrow the median ratio of measured
// models of the same kind and mode, so one benchmark already improves every
// estimate.
package bench

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/llamafile-ui/internal/catalog"
	"github.com/devalexandre/llamafile-ui/internal/hardware"
)

// alpha weighs a new sample against the history (exponential moving average).
const alpha = 0.35

type Record struct {
	ModelID   string  `json:"modelId"`
	Quant     string  `json:"quant"`
	Kind      string  `json:"kind"` // llm, image, voice
	Mode      string  `json:"mode"` // GPU, GPU+CPU, CPU, Metal
	Ratio     float64 `json:"ratio"`
	Value     float64 `json:"value"`     // last measured metric (tok/s, s/image, realtime factor)
	PromptTPS float64 `json:"promptTps"` // LLM prompt processing speed, when known
	Samples   int     `json:"samples"`
	UpdatedAt int64   `json:"updatedAt"`
}

// Capability is what a tool build can do, detected from its logs.
type Capability struct {
	Tool      string   `json:"tool"`
	Version   string   `json:"version"`
	Backends  []string `json:"backends"`
	GPU       bool     `json:"gpu"`
	CheckedAt int64    `json:"checkedAt"`
}

type file struct {
	Hardware     string                `json:"hardware"`
	Records      map[string]Record     `json:"records"`
	Capabilities map[string]Capability `json:"capabilities"`
}

type Store struct {
	mu   sync.Mutex
	path string
	data file
}

// Fingerprint identifies the hardware measurements belong to; when it
// changes (new GPU, more RAM…) old measurements no longer apply.
func Fingerprint(hw hardware.Info) string {
	return fmt.Sprintf("%s|%s|%.0f", hw.CPUName, hw.GPUName, math.Round(hw.TotalRAMGB))
}

func Open(path string, hw hardware.Info) *Store {
	s := &Store{path: path}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &s.data)
	}
	if s.data.Records == nil || s.data.Hardware != Fingerprint(hw) {
		s.data.Records = map[string]Record{}
	}
	if s.data.Capabilities == nil {
		s.data.Capabilities = map[string]Capability{}
	}
	s.data.Hardware = Fingerprint(hw)
	return s
}

func key(modelID, quant, mode string) string { return modelID + "|" + quant + "|" + mode }

func (s *Store) save() {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err == nil {
		_ = os.WriteFile(s.path, raw, 0o644)
	}
}

// Observe records a measurement. measured and estimated use the same unit;
// promptTPS is optional (0 when unknown).
func (s *Store) Observe(modelID, quant, kind, mode string, measured, estimated, promptTPS float64) Record {
	if measured <= 0 || estimated <= 0 || math.IsInf(measured, 0) || math.IsNaN(measured) {
		return Record{}
	}
	ratio := measured / estimated
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(modelID, quant, mode)
	r, ok := s.data.Records[k]
	if !ok {
		r = Record{ModelID: modelID, Quant: quant, Kind: kind, Mode: mode, Ratio: ratio}
	} else {
		r.Ratio = r.Ratio*(1-alpha) + ratio*alpha
	}
	r.Value = measured
	if promptTPS > 0 {
		r.PromptTPS = promptTPS
	}
	r.Samples++
	r.UpdatedAt = time.Now().UnixMilli()
	s.data.Records[k] = r
	s.save()
	return r
}

// Lookup returns the measurement of a model, if any.
func (s *Store) Lookup(modelID, quant, mode string) (Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.data.Records[key(modelID, quant, mode)]
	return r, ok
}

// Records lists every measurement, most recent first.
func (s *Store) Records() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, 0, len(s.data.Records))
	for _, r := range s.data.Records {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}

// Reset forgets every measurement.
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Records = map[string]Record{}
	s.save()
}

// Calibrator plugs the measurements into the fit engine.
func (s *Store) Calibrator() catalog.Calibrator {
	return func(modelID, quant, kind, mode string) (catalog.Calibration, bool) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if r, ok := s.data.Records[key(modelID, quant, mode)]; ok {
			return catalog.Calibration{Ratio: r.Ratio, Measured: true, Samples: r.Samples}, true
		}
		// Median ratio of other models of the same kind and mode.
		var ratios []float64
		samples := 0
		for _, r := range s.data.Records {
			if r.Kind == kind && r.Mode == mode {
				ratios = append(ratios, r.Ratio)
				samples += r.Samples
			}
		}
		if len(ratios) == 0 {
			return catalog.Calibration{}, false
		}
		sort.Float64s(ratios)
		median := ratios[len(ratios)/2]
		if len(ratios)%2 == 0 {
			median = (ratios[len(ratios)/2-1] + ratios[len(ratios)/2]) / 2
		}
		return catalog.Calibration{Ratio: median, Samples: samples}, true
	}
}

// SetCapability records what a tool build supports.
func (s *Store) SetCapability(tool, version string, backends []string) Capability {
	gpu := false
	for _, b := range backends {
		if !strings.EqualFold(b, "CPU") {
			gpu = true
		}
	}
	c := Capability{Tool: tool, Version: version, Backends: backends, GPU: gpu, CheckedAt: time.Now().UnixMilli()}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Capabilities[tool] = c
	s.save()
	return c
}

// Capability returns what is known about a tool build (ok=false if unknown or
// detected for another version).
func (s *Store) Capability(tool, version string) (Capability, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.data.Capabilities[tool]
	if !ok || c.Version != version {
		return Capability{}, false
	}
	return c, true
}
