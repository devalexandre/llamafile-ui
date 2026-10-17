package tools

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	whisperBackendRe = regexp.MustCompile(`registered backend (\S+)`)
	diffusionDevRe   = regexp.MustCompile(`#\d+: (.+)`)
	diffusionUsingRe = regexp.MustCompile(`Using (\S+) backend`)
	ggmlLoadedRe     = regexp.MustCompile(`loaded (\S+) backend from`) // newer ggml builds with dynamic backends
)

// ProbeBackends runs a whisperfile or diffusionfile build for a fraction of a
// second and returns the compute backends it registered (e.g. ["CPU"] or
// ["CUDA", "CPU"]). whisperfile only logs its backends once a model loads, so
// it needs a Whisper model; diffusionfile logs them before opening the model.
func ProbeBackends(ctx context.Context, t Tool, bin, whisperModel string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	tmp, err := os.MkdirTemp("", "llamafile-ui-probe")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	var args []string
	switch t {
	case Whisper:
		if whisperModel == "" {
			return nil, errors.New("é preciso um modelo Whisper para detectar a GPU do whisperfile")
		}
		wav := filepath.Join(tmp, "probe.wav")
		if err := writeSilence(wav, 16000); err != nil {
			return nil, err
		}
		// -d 100 decodes only 100 ms: the run takes well under a second.
		args = []string{"-m", whisperModel, "-f", wav, "-d", "100", "-t", "1", "-l", "en"}
	case Diffusion:
		args = []string{"-v", "-m", filepath.Join(tmp, "missing.gguf"), "-p", "probe", "-o", filepath.Join(tmp, "probe.png")}
	default:
		return nil, errors.New("ferramenta sem detecção de backend")
	}
	out, _ := Command(ctx, bin, args...).CombinedOutput() // a non-zero exit is expected for diffusionfile
	backends := parseBackends(t, string(out))
	if len(backends) == 0 {
		return nil, errors.New("não foi possível identificar os backends da ferramenta")
	}
	return backends, nil
}

func parseBackends(t Tool, log string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" && !seen[strings.ToLower(name)] {
			seen[strings.ToLower(name)] = true
			out = append(out, name)
		}
	}
	switch t {
	case Whisper:
		for _, m := range whisperBackendRe.FindAllStringSubmatch(log, -1) {
			add(m[1])
		}
	case Diffusion:
		for _, m := range diffusionUsingRe.FindAllStringSubmatch(log, -1) {
			add(m[1])
		}
		for _, m := range ggmlLoadedRe.FindAllStringSubmatch(log, -1) {
			add(m[1])
		}
		for _, m := range diffusionDevRe.FindAllStringSubmatch(log, -1) {
			add(m[1])
		}
	}
	return out
}

// writeSilence writes one second of 16-bit mono silence.
func writeSilence(path string, rate uint32) error {
	data := make([]byte, rate*2)
	hdr := make([]byte, 44)
	copy(hdr[0:], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(36+len(data)))
	copy(hdr[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(hdr[16:], 16)
	binary.LittleEndian.PutUint16(hdr[20:], 1)
	binary.LittleEndian.PutUint16(hdr[22:], 1)
	binary.LittleEndian.PutUint32(hdr[24:], rate)
	binary.LittleEndian.PutUint32(hdr[28:], rate*2)
	binary.LittleEndian.PutUint16(hdr[32:], 2)
	binary.LittleEndian.PutUint16(hdr[34:], 16)
	copy(hdr[36:], "data")
	binary.LittleEndian.PutUint32(hdr[40:], uint32(len(data)))
	return os.WriteFile(path, append(hdr, data...), 0o644)
}

// Newer reports whether version a is newer than b (e.g. "0.10.7" > "0.10.6").
func Newer(a, b string) bool { return compareVersions(a, b) > 0 }
