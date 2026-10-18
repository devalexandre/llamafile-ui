// Package voice records audio from the microphone and transcribes it with
// whisperfile.
package voice

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"sync"
	"time"

	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"github.com/gen2brain/malgo"
)

// Whisper expects 16 kHz mono PCM.
const sampleRate = 16000

// Recorder captures microphone audio. Only one recording at a time.
type Recorder struct {
	mu      sync.Mutex
	ctx     *malgo.AllocatedContext
	device  *malgo.Device
	pcm     []byte
	started time.Time
	level   float64

	// OnLevel receives the input level (0..1) about 12 times per second.
	OnLevel  func(level float64, elapsed time.Duration)
	stopTick chan struct{}
}

func (r *Recorder) Recording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.device != nil
}

// Start opens the default capture device.
func (r *Recorder) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.device != nil {
		return errors.New(i18n.T("já está gravando"))
	}
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return errors.New(i18n.Tf("não foi possível acessar o áudio do sistema: %s", err))
	}
	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = 1
	cfg.SampleRate = sampleRate

	r.pcm = r.pcm[:0]
	onData := func(_, input []byte, _ uint32) {
		r.mu.Lock()
		r.pcm = append(r.pcm, input...)
		// RMS of this buffer for the UI meter.
		var sum float64
		n := len(input) / 2
		for i := 0; i+1 < len(input); i += 2 {
			s := float64(int16(binary.LittleEndian.Uint16(input[i:]))) / 32768
			sum += s * s
		}
		if n > 0 {
			rms := math.Sqrt(sum / float64(n))
			r.level = math.Max(rms*4, r.level*0.85) // fast attack, slow release
		}
		r.mu.Unlock()
	}
	dev, err := malgo.InitDevice(ctx.Context, cfg, malgo.DeviceCallbacks{Data: onData})
	if err != nil {
		_ = ctx.Uninit()
		ctx.Free()
		return errors.New(i18n.Tf("nenhum microfone disponível: %s", err))
	}
	if err := dev.Start(); err != nil {
		dev.Uninit()
		_ = ctx.Uninit()
		ctx.Free()
		return errors.New(i18n.Tf("não foi possível iniciar o microfone: %s", err))
	}
	r.ctx, r.device, r.started, r.level = ctx, dev, time.Now(), 0
	r.stopTick = make(chan struct{})
	go r.tick(r.stopTick)
	return nil
}

func (r *Recorder) tick(stop chan struct{}) {
	t := time.NewTicker(80 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			r.mu.Lock()
			level, elapsed := math.Min(r.level, 1), time.Since(r.started)
			r.level *= 0.9
			r.mu.Unlock()
			if r.OnLevel != nil {
				r.OnLevel(level, elapsed)
			}
		}
	}
}

func (r *Recorder) release() {
	if r.stopTick != nil {
		close(r.stopTick)
		r.stopTick = nil
	}
	if r.device != nil {
		r.device.Uninit()
		r.device = nil
	}
	if r.ctx != nil {
		_ = r.ctx.Uninit()
		r.ctx.Free()
		r.ctx = nil
	}
}

// Stop ends the recording and writes it as a WAV file at path.
// It returns the recorded duration.
func (r *Recorder) Stop(path string) (time.Duration, error) {
	r.mu.Lock()
	if r.device == nil {
		r.mu.Unlock()
		return 0, errors.New(i18n.T("nenhuma gravação em andamento"))
	}
	dev := r.device
	r.mu.Unlock()
	_ = dev.Stop() // outside the lock: the data callback takes it

	r.mu.Lock()
	r.release()
	pcm := append([]byte(nil), r.pcm...)
	r.mu.Unlock()

	duration := time.Duration(len(pcm)/2) * time.Second / sampleRate
	if duration < 300*time.Millisecond {
		return duration, errors.New(i18n.T("gravação curta demais"))
	}
	return duration, writeWAV(path, pcm)
}

// Cancel discards the current recording.
func (r *Recorder) Cancel() {
	r.mu.Lock()
	dev := r.device
	r.mu.Unlock()
	if dev == nil {
		return
	}
	_ = dev.Stop()
	r.mu.Lock()
	r.release()
	r.pcm = r.pcm[:0]
	r.mu.Unlock()
}

func writeWAV(path string, pcm []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := make([]byte, 44)
	copy(h[0:], "RIFF")
	binary.LittleEndian.PutUint32(h[4:], uint32(36+len(pcm)))
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)           // PCM chunk size
	binary.LittleEndian.PutUint16(h[20:], 1)            // PCM
	binary.LittleEndian.PutUint16(h[22:], 1)            // mono
	binary.LittleEndian.PutUint32(h[24:], sampleRate)   // sample rate
	binary.LittleEndian.PutUint32(h[28:], sampleRate*2) // byte rate
	binary.LittleEndian.PutUint16(h[32:], 2)            // block align
	binary.LittleEndian.PutUint16(h[34:], 16)           // bits per sample
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], uint32(len(pcm)))
	if _, err := f.Write(h); err != nil {
		return err
	}
	_, err = f.Write(pcm)
	return err
}
