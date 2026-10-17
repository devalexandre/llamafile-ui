package voice

import (
	"context"
	"os"
	"testing"
	"time"
)

// LIVE_BIN=… LIVE_MODEL=… LIVE_AUDIO=… go test -run TestLiveTranscribe ./internal/voice
func TestLiveTranscribe(t *testing.T) {
	bin, model, audio := os.Getenv("LIVE_BIN"), os.Getenv("LIVE_MODEL"), os.Getenv("LIVE_AUDIO")
	if bin == "" {
		t.Skip("set LIVE_BIN, LIVE_MODEL and LIVE_AUDIO")
	}
	start := time.Now()
	res, err := Transcribe(context.Background(), audio, Options{Binary: bin, Model: model, Language: "pt"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%v: %.120s", time.Since(start).Round(100*time.Millisecond), res.Text)
}
