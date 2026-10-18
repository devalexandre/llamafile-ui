package tts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gen2brain/malgo"

	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"github.com/devalexandre/llamafile-ui/internal/sysproc"
)

// Options configures one reading.
type Options struct {
	Binary string  // piper executable
	Model  string  // voice .onnx (its config is <model>.json)
	Speed  float64 // 1 = normal, 1.5 = 50% faster
}

// Speaker plays one text at a time; a new Speak interrupts the current one.
type Speaker struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	id     string

	// OnState is called when a reading starts (playing=true) and when it
	// ends, is interrupted or fails (playing=false, err set on failure).
	OnState func(id string, playing bool, err string)
}

// Current returns the id of the text being read, or "".
func (s *Speaker) Current() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id
}

// Speak starts reading text aloud in the background. id identifies the text
// for the UI (e.g. a chat message).
func (s *Speaker) Speak(id, text string, o Options) error {
	text = Clean(text)
	if text == "" {
		return errors.New(i18n.T("não há texto para ler"))
	}
	if o.Binary == "" || o.Model == "" {
		return errors.New(i18n.T("instale o Piper e uma voz para ouvir as respostas"))
	}
	if o.Speed <= 0 {
		o.Speed = 1
	}
	s.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel, s.id = cancel, id
	s.mu.Unlock()
	s.notify(id, true, "")
	go func() {
		err := play(ctx, text, o)
		s.mu.Lock()
		if s.id == id {
			s.cancel, s.id = nil, ""
		}
		s.mu.Unlock()
		cancel()
		msg := ""
		if err != nil && ctx.Err() == nil {
			msg = err.Error()
		}
		s.notify(id, false, msg)
	}()
	return nil
}

// Stop interrupts the current reading.
func (s *Speaker) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel, s.id = nil, ""
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Speaker) notify(id string, playing bool, err string) {
	if s.OnState != nil {
		s.OnState(id, playing, err)
	}
}

// pcm is the audio produced by piper and not played yet.
type pcm struct {
	mu   sync.Mutex
	data []byte
	eof  bool
}

func (p *pcm) Write(b []byte) (int, error) {
	p.mu.Lock()
	p.data = append(p.data, b...)
	p.mu.Unlock()
	return len(b), nil
}

// fill copies queued audio into out and pads with silence.
func (p *pcm) fill(out []byte) {
	p.mu.Lock()
	n := copy(out, p.data)
	p.data = p.data[n:]
	p.mu.Unlock()
	clear(out[n:])
}

func (p *pcm) drained() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.eof && len(p.data) == 0
}

func command(ctx context.Context, text string, o Options) *exec.Cmd {
	dir := filepath.Dir(o.Binary)
	cmd := exec.CommandContext(ctx, o.Binary, "--model", o.Model, "--output_raw", "--quiet",
		"--length_scale", strconv.FormatFloat(1/o.Speed, 'f', 3, 64), "--sentence_silence", "0.3",
		"--espeak_data", filepath.Join(dir, "espeak-ng-data"))
	cmd.Dir = dir
	cmd.Env = os.Environ()
	switch goruntime.GOOS {
	case "linux":
		cmd.Env = append(cmd.Env, "LD_LIBRARY_PATH="+dir)
	case "darwin":
		cmd.Env = append(cmd.Env, "DYLD_LIBRARY_PATH="+dir)
	}
	cmd.Stdin = strings.NewReader(text + "\n")
	sysproc.Prepare(cmd)
	return cmd
}

func play(ctx context.Context, text string, o Options) error {
	buf := &pcm{}
	cmd := command(ctx, text, o)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf(i18n.T("não foi possível iniciar o Piper: %w"), err)
	}
	copied := make(chan error, 1)
	go func() {
		_, err := io.Copy(buf, stdout)
		buf.mu.Lock()
		buf.eof = true
		buf.mu.Unlock()
		copied <- err
	}()

	mctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		_ = cmd.Process.Kill()
		return errors.New(i18n.Tf("não foi possível acessar o áudio do sistema: %s", err))
	}
	defer func() {
		_ = mctx.Uninit()
		mctx.Free()
	}()
	cfg := malgo.DefaultDeviceConfig(malgo.Playback)
	cfg.Playback.Format = malgo.FormatS16
	cfg.Playback.Channels = 1
	cfg.SampleRate = uint32(SampleRate(o.Model))
	dev, err := malgo.InitDevice(mctx.Context, cfg, malgo.DeviceCallbacks{
		Data: func(out, _ []byte, _ uint32) { buf.fill(out) },
	})
	if err != nil {
		_ = cmd.Process.Kill()
		return errors.New(i18n.Tf("nenhuma saída de áudio disponível: %s", err))
	}
	defer dev.Uninit()
	if err := dev.Start(); err != nil {
		_ = cmd.Process.Kill()
		return errors.New(i18n.Tf("não foi possível tocar o áudio: %s", err))
	}

	tick := time.NewTicker(40 * time.Millisecond)
	defer tick.Stop()
	for !buf.drained() {
		select {
		case <-ctx.Done():
			_ = dev.Stop()
			_ = cmd.Wait()
			return ctx.Err()
		case <-tick.C:
		}
	}
	time.Sleep(250 * time.Millisecond) // let the device play its last buffer
	_ = dev.Stop()
	<-copied
	if err := cmd.Wait(); err != nil && ctx.Err() == nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("Piper: %s", lastLine(msg))
	}
	return nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}
