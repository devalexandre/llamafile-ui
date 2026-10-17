package main

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/devalexandre/llamafile-ui/internal/catalog"
	"github.com/devalexandre/llamafile-ui/internal/downloads"
	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/settings"
	"github.com/devalexandre/llamafile-ui/internal/tools"
	"github.com/devalexandre/llamafile-ui/internal/tts"
)

// piperTool is listed with the llamafile tools but comes from the Piper project.
const piperTool tools.Tool = "piper"

func (a *App) piperStatus() ToolStatus {
	bin := tts.Binary()
	return ToolStatus{Tool: string(piperTool), Installed: bin != "", Path: bin, Version: tts.Version, Backends: []string{},
		Downloading: a.downloads.IsActive(tools.DownloadKey(piperTool))}
}

func (a *App) downloadPiper() error {
	asset, ok := tts.ReleaseAsset()
	if !ok {
		return errors.New("o Piper não tem versão pronta para este sistema")
	}
	key := tools.DownloadKey(piperTool)
	dest := filepath.Join(paths.BinDir(), asset.Name)
	a.mu.Lock()
	a.pendingTools[key] = dest
	a.mu.Unlock()
	a.emit("tools:changed")
	return a.downloads.Start(key, "Piper "+tts.Version, []downloads.File{{URL: asset.URL, Dest: dest, Size: asset.Size}})
}

// ttsVoice returns the chosen (or first installed) Piper voice.
func (a *App) ttsVoice(id string) (catalog.View, error) {
	if id == "" {
		id = a.settings.Get().TTSVoiceID
	}
	list := a.installedFor("tts", id)
	if len(list) == 0 {
		return catalog.View{}, errors.New("baixe uma voz em Descobrir → Leitura em voz alta")
	}
	return list[0], nil
}

func (a *App) speak(id, text, voiceID string) error {
	bin := tts.Binary()
	if bin == "" {
		return errors.New("instale o Piper em Configurações → Ferramentas para ouvir as respostas")
	}
	v, err := a.ttsVoice(voiceID)
	if err != nil {
		return err
	}
	return a.speaker.Speak(id, text, tts.Options{Binary: bin, Model: a.catalog.ModelPath(v.ID), Speed: a.settings.Get().TTSSpeed})
}

// Speak reads a text aloud (e.g. a chat answer); id lets the UI show which
// message is playing. A new call interrupts the current reading.
func (a *App) Speak(id, text string) error { return a.speak(id, text, "") }

func (a *App) StopSpeaking() { a.speaker.Stop() }

// SpeakingID is the id passed to the reading in progress, or "".
func (a *App) SpeakingID() string { return a.speaker.Current() }

var voiceSamples = map[string]string{
	"pt": "Olá! Esta é a voz que vai ler as respostas do chat para você.",
	"en": "Hello! This is the voice that will read the chat answers to you.",
	"es": "¡Hola! Esta es la voz que leerá las respuestas del chat para ti.",
}

// PreviewVoice plays a short sample with an installed voice.
func (a *App) PreviewVoice(voiceID string) error {
	m := a.catalog.Find(voiceID)
	sample := voiceSamples["pt"]
	if m != nil && m.TTS != nil {
		if s, ok := voiceSamples[strings.ToLower(strings.SplitN(m.TTS.Language, "-", 2)[0])]; ok {
			sample = s
		}
	}
	return a.speak("preview:"+voiceID, sample, voiceID)
}

func (a *App) SetTTSVoice(id string) error {
	return a.updateSettings(func(s *settings.Settings) { s.TTSVoiceID = id })
}
