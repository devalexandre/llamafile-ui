//go:build integration

package voice

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/tools"
)

// go test -tags integration ./internal/voice -v   (VOICE_SAMPLE=/path/to/speech.wav)
func TestIntegrationTranscribe(t *testing.T) {
	sample := os.Getenv("VOICE_SAMPLE")
	bin, _ := tools.Find(tools.Whisper, "")
	model := filepath.Join(paths.DefaultModelsDir(), "ggml-base-q5_1.bin")
	if sample == "" || bin == "" {
		t.Skip("set VOICE_SAMPLE and install whisperfile")
	}
	last := -1
	res, err := Transcribe(context.Background(), sample, Options{Binary: bin, Model: model, Language: "auto",
		OnProgress: func(p int) { last = p }})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("progress=%d text=%q srtLines=%d", last, res.Text, len(res.SRT))
	if res.Text == "" {
		t.Fatal("empty transcript")
	}
}

func TestIntegrationRecord(t *testing.T) {
	var r Recorder
	levels := 0
	r.OnLevel = func(float64, time.Duration) { levels++ }
	if err := r.Start(); err != nil {
		t.Skip("no microphone: ", err)
	}
	time.Sleep(1500 * time.Millisecond)
	out := filepath.Join(t.TempDir(), "rec.wav")
	d, err := r.Stop(out)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(out)
	t.Logf("recorded %v, %d bytes, %d level ticks", d, st.Size(), levels)
}
