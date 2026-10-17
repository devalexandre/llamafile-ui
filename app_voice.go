package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devalexandre/llamafile-ui/internal/catalog"
	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/settings"
	"github.com/devalexandre/llamafile-ui/internal/tools"
	"github.com/devalexandre/llamafile-ui/internal/voice"
)

// voiceModel returns the chosen (or best installed) Whisper model.
func (a *App) voiceModel() (catalog.View, error) {
	list := a.installedFor("voice", a.settings.Get().VoiceModelID)
	if len(list) == 0 {
		return catalog.View{}, errors.New("baixe um modelo de voz (Whisper) em Descobrir → Voz")
	}
	return list[0], nil
}

func (a *App) SetVoiceModel(id string) error {
	return a.updateSettings(func(s *settings.Settings) { s.VoiceModelID = id })
}

func (a *App) SetVoiceLanguage(lang string) error {
	return a.updateSettings(func(s *settings.Settings) { s.VoiceLanguage = lang })
}

func (a *App) StartRecording() error {
	if a.toolPath(tools.Whisper) == "" {
		return errors.New("instale o whisperfile para usar voz")
	}
	if _, err := a.voiceModel(); err != nil {
		return err
	}
	return a.recorder.Start()
}

func (a *App) CancelRecording() { a.recorder.Cancel() }

func (a *App) CancelTranscription() {
	a.mu.Lock()
	cancel := a.transcribeAbort
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// transcribe runs whisperfile with progress events; one job at a time.
func (a *App) transcribe(audio string) (voice.Result, catalog.View, time.Duration, error) {
	m, err := a.voiceModel()
	if err != nil {
		return voice.Result{}, m, 0, err
	}
	bin := a.toolPath(tools.Whisper)
	if bin == "" {
		return voice.Result{}, m, 0, errors.New("instale o whisperfile para usar voz")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	if a.transcribeAbort != nil {
		a.mu.Unlock()
		cancel()
		return voice.Result{}, m, 0, errors.New("já existe uma transcrição em andamento")
	}
	a.transcribeAbort = cancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.transcribeAbort = nil
		a.mu.Unlock()
		cancel()
	}()

	started := time.Now()
	a.emit("voice:progress", 0)
	res, err := voice.Transcribe(ctx, audio, voice.Options{
		Binary: bin, Model: a.catalog.ModelPath(m.ID), Language: a.settings.Get().VoiceLanguage,
		OnProgress: func(p int) { a.emit("voice:progress", p) },
	})
	if errors.Is(err, context.Canceled) {
		err = errors.New("transcrição cancelada")
	}
	return res, m, time.Since(started), err
}

// StopRecording ends the microphone capture and transcribes it. When save is
// true the transcript is stored in the history (Transcription page);
// otherwise it is only returned (chat dictation).
func (a *App) StopRecording(save bool) (voice.Transcript, error) {
	wav := filepath.Join(paths.RecordingsDir(), time.Now().Format("2006-01-02_15-04-05")+".wav")
	if _, err := a.recorder.Stop(wav); err != nil {
		_ = os.Remove(wav)
		return voice.Transcript{}, err
	}
	res, m, took, err := a.transcribe(wav)
	_ = os.Remove(wav) // only the text is kept
	if err != nil {
		return voice.Transcript{}, err
	}
	t := voice.Transcript{Name: "Gravação " + time.Now().Format("02/01 15:04"), Source: "recording", ModelID: m.ID,
		Language: a.settings.Get().VoiceLanguage, DurationMs: took.Milliseconds(), Text: res.Text, SRT: res.SRT}
	if !save {
		return t, nil
	}
	return a.voice.Add(t)
}

func (a *App) PickAudioFiles() ([]string, error) {
	return a.openFiles("Escolha arquivos de áudio", fileFilter{"Áudio (WAV, MP3, OGG, FLAC)", "*.wav;*.mp3;*.ogg;*.flac"})
}

// TranscribeFile transcribes an audio file and stores the result.
func (a *App) TranscribeFile(path string) (voice.Transcript, error) {
	res, m, took, err := a.transcribe(path)
	if err != nil {
		return voice.Transcript{}, err
	}
	return a.voice.Add(voice.Transcript{Name: filepath.Base(path), Source: "file", ModelID: m.ID,
		Language: a.settings.Get().VoiceLanguage, DurationMs: took.Milliseconds(), Text: res.Text, SRT: res.SRT})
}

func (a *App) ListTranscripts() ([]voice.Transcript, error) { return a.voice.List() }
func (a *App) DeleteTranscript(id int64) error              { return a.voice.Delete(id) }

// ExportTranscript saves a transcript as .txt or .srt.
func (a *App) ExportTranscript(id int64, format string) (string, error) {
	t, err := a.voice.Get(id)
	if err != nil {
		return "", err
	}
	base := strings.TrimSuffix(t.Name, filepath.Ext(t.Name))
	if format == "srt" {
		if t.SRT == "" {
			return "", fmt.Errorf("esta transcrição não tem legendas")
		}
		return a.saveAs("Exportar legendas", base+".srt", []byte(t.SRT))
	}
	return a.saveAs("Exportar transcrição", base+".txt", []byte(t.Text))
}
