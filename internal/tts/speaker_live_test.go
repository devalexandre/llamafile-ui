package tts

import (
	"context"
	"os"
	"testing"
	"time"
)

// PIPER_BIN=… PIPER_VOICE=….onnx go test -run TestLive ./internal/tts
func TestLiveSynthesis(t *testing.T) {
	bin, voice := os.Getenv("PIPER_BIN"), os.Getenv("PIPER_VOICE")
	if bin == "" || voice == "" {
		t.Skip("set PIPER_BIN and PIPER_VOICE")
	}
	buf := &pcm{}
	cmd := command(context.Background(), Clean("**Olá!** Este é um teste do Llamafile UI."), Options{Binary: bin, Model: voice, Speed: 1})
	cmd.Stdout = buf
	start := time.Now()
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	secs := float64(len(buf.data)) / 2 / float64(SampleRate(voice))
	t.Logf("%.1f s de áudio em %v", secs, time.Since(start))
	if secs < 1 {
		t.Fatal("áudio curto demais")
	}
}
