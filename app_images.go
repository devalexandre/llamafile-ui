package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"github.com/devalexandre/llamafile-ui/internal/imagegen"
	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/settings"
)

// ------------------------------------------------------------------ images

// ImageDefaults are the recommended settings for a model.
type ImageDefaults struct {
	Steps    int     `json:"steps"`
	CFG      float64 `json:"cfg"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Sampler  string  `json:"sampler"`
	Negative bool    `json:"negative"` // model benefits from a negative prompt
	TAESD    bool    `json:"taesd"`
	// Video models only.
	Video      bool `json:"video"`
	Frames     int  `json:"frames"`
	FPS        int  `json:"fps"`
	ImageInput bool `json:"imageInput"` // can animate a reference image
}

func (a *App) GetImageDefaults(modelID string) (ImageDefaults, error) {
	m := a.catalog.Find(modelID)
	if m == nil || m.Image == nil {
		return ImageDefaults{}, errors.New(i18n.T("modelo de imagem desconhecido"))
	}
	_, taesd := a.catalog.ComponentPaths(modelID)["taesd"]
	d := ImageDefaults{Steps: m.Image.Steps, CFG: m.Image.CFG, Width: m.Image.Width, Height: m.Image.Height,
		Sampler: m.Image.Sampler, Negative: m.Image.CFG > 1, TAESD: taesd}
	if m.Video != nil {
		d.Video, d.Frames, d.FPS, d.ImageInput = true, m.Video.Frames, m.Video.FPS, m.Video.ImageInput
	}
	return d, nil
}

func (a *App) EstimateImage(req imagegen.Request) float64 { return (&backend{a}).Estimate(req) }

func (a *App) GenerateImage(req imagegen.Request) (string, error) {
	category, chosen := "image", a.settings.Get().ImageModelID
	if req.Video {
		category, chosen = "video", a.settings.Get().VideoModelID
	}
	if req.ModelID == "" {
		list := a.installedFor(category, chosen)
		if len(list) == 0 {
			if req.Video {
				return "", errors.New(i18n.T("baixe um modelo de vídeo em Descobrir → Vídeos"))
			}
			return "", errors.New(i18n.T("baixe um modelo de imagem em Descobrir → Imagens"))
		}
		req.ModelID = list[0].ID
	}
	if m := a.catalog.Find(req.ModelID); m != nil {
		req.Video = m.Video != nil // the model decides
		if req.Video && req.InitImage != "" && !m.Video.ImageInput {
			return "", errors.New(i18n.Tf("%s não usa imagem de referência; escolha o Wan 2.2 5B Turbo para animar uma imagem", m.Name))
		}
	}
	req.InitImage = a.referencePath(req.InitImage)
	_ = a.updateSettings(func(s *settings.Settings) {
		if req.Video {
			s.VideoModelID = req.ModelID
		} else {
			s.ImageModelID = req.ModelID
			s.FastDecode = req.FastDecode
		}
	})
	return a.images.Generate(req)
}

// referencePath resolves a reference image given as a chat attachment name.
func (a *App) referencePath(ref string) string {
	if ref == "" || filepath.IsAbs(ref) {
		return ref
	}
	return filepath.Join(paths.AttachmentsDir(), filepath.Base(ref))
}

func (a *App) CancelImage()                          { a.images.Cancel() }
func (a *App) GetImageJob() string                   { return a.images.Busy() }
func (a *App) ListImages() ([]imagegen.Image, error) { return a.images.List() }
func (a *App) DeleteImage(id int64) error            { return a.images.Delete(id) }

// SaveImageAs exports a generated image with a file dialog.
func (a *App) SaveImageAs(id int64) (string, error) {
	img, err := a.images.Get(id)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(a.images.Path(img.File))
	if err != nil {
		return "", err
	}
	name := strings.Join(strings.Fields(img.Prompt), " ")
	if r := []rune(name); len(r) > 40 {
		name = string(r[:40])
	}
	return a.saveAs(i18n.T("Salvar imagem"), name+".png", data)
}

// ImageToAttachment copies a generated image into the chat attachments.
func (a *App) ImageToAttachment(id int64) (string, error) {
	img, err := a.images.Get(id)
	if err != nil {
		return "", err
	}
	return copyAttachment(a.images.Path(img.File))
}

// EnhanceImagePrompt uses the local chat model to turn a description (in any
// language) into a detailed English prompt, which image models follow better.
func (a *App) EnhanceImagePrompt(prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New(i18n.T("escreva uma descrição primeiro"))
	}
	modelID := a.settings.Get().LastModelID
	if m := a.catalog.Find(modelID); m == nil || !m.IsChat() || a.catalog.ModelPath(modelID) == "" {
		modelID = ""
		for _, v := range a.catalog.Installed(a.fitHardware(), a.ctxSize()) {
			if v.IsChat() {
				modelID = v.ID
				break
			}
		}
	}
	if modelID == "" {
		return "", errors.New(i18n.T("baixe um modelo de chat para melhorar prompts"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := a.runtime.EnsureModel(ctx, modelID); err != nil {
		return "", err
	}
	out, err := a.complete(ctx, modelID,
		"You write prompts for Stable Diffusion image models. Rewrite the user's idea as ONE English prompt: "+
			"subject first, then setting, style, lighting and quality keywords, comma-separated, at most 60 words. "+
			"Reply with the prompt only.", prompt, 160)
	if err != nil {
		return "", err
	}
	out = strings.Trim(strings.TrimSpace(out), "\"'`")
	if out == "" {
		return "", errors.New(i18n.T("o modelo não retornou um prompt"))
	}
	return out, nil
}
