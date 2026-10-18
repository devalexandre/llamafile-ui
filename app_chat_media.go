package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/devalexandre/llamafile-ui/internal/catalog"
	"github.com/devalexandre/llamafile-ui/internal/chat"
	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"github.com/devalexandre/llamafile-ui/internal/imagegen"
	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/rag"
	"github.com/devalexandre/llamafile-ui/internal/voice"
)

// Everything the chat composer accepts: images, documents and audio.
var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true}

// PickChatFiles opens a dialog for images, documents and audio files.
// Audio is transcribed into the message; the rest is attached.
func (a *App) PickChatFiles() ([]string, error) {
	var docs []string
	for _, e := range rag.SupportedExtensions() {
		docs = append(docs, "*"+e)
	}
	var audio []string
	for _, e := range voice.SupportedExtensions {
		audio = append(audio, "*"+e)
	}
	all := append(append([]string{"*.png", "*.jpg", "*.jpeg", "*.webp", "*.gif"}, docs...), audio...)
	return a.openFiles(i18n.T("Anexar arquivos"),
		fileFilter{i18n.T("Imagens, documentos e áudio"), strings.Join(all, ";")},
		fileFilter{i18n.T("Imagens"), "*.png;*.jpg;*.jpeg;*.webp;*.gif"},
		fileFilter{i18n.T("Documentos (PDF, DOCX, texto, código)"), strings.Join(docs, ";")},
		fileFilter{i18n.T("Áudio"), strings.Join(audio, ";")},
	)
}

// AttachFiles turns local files into chat attachments: images are copied,
// documents have their text extracted. Audio must go through TranscribeForChat.
func (a *App) AttachFiles(paths []string) ([]string, error) {
	names := []string{}
	for _, p := range paths {
		ext := strings.ToLower(filepath.Ext(p))
		switch {
		case imageExts[ext]:
			name, err := copyAttachment(p)
			if err != nil {
				return names, err
			}
			names = append(names, name)
		case rag.Supported(p):
			name, err := attachDocument(p)
			if err != nil {
				return names, fmt.Errorf("%s: %w", filepath.Base(p), err)
			}
			names = append(names, name)
		case voice.Supported(p):
			return names, errors.New(i18n.T("arquivos de áudio são transcritos para o texto da mensagem"))
		default:
			return names, errors.New(i18n.Tf("%s: formato não suportado no chat", filepath.Base(p)))
		}
	}
	return names, nil
}

func attachDocument(path string) (string, error) {
	text, err := rag.ExtractText(path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", errors.New(i18n.T("nenhum texto encontrado no arquivo"))
	}
	base := strings.NewReplacer("/", "_", "\\", "_").Replace(filepath.Base(path))
	name := uuid.NewString() + "__" + base + chat.DocumentSuffix
	return name, os.WriteFile(filepath.Join(paths.AttachmentsDir(), name), []byte(text), 0o644)
}

// TranscribeForChat transcribes an audio file for the composer (not saved in
// the Transcription history).
func (a *App) TranscribeForChat(path string) (string, error) {
	res, _, _, err := a.transcribe(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Text), nil
}

// ------------------------------------------------------- images in chat

var imageIntentRe = regexp.MustCompile(`(?i)^\s*(por favor,?\s*)?(crie|cria|criar|gere|gera|gerar|desenhe|desenha|fa[çc]a|faz|pinte|ilustre|create|generate|draw|make|paint)\b.{0,40}?\b(imagem|imagens|foto|fotografia|ilustra[çc][ãa]o|desenho|pintura|arte|logo|image|picture|photo|illustration|drawing|painting)\b`)

var videoIntentRe = regexp.MustCompile(`(?i)\b(v[ií]deo|anima[çc][ãa]o|clipe|video|animation|clip)\b|^\s*(anime|animate)\b`)

// WantsVideo reports whether a chat message asks for a video ("crie um vídeo de…", "anime esta imagem").
func (a *App) WantsVideo(text string) bool { return videoIntentRe.MatchString(text) }

// WantsImage reports whether a chat message asks for an image ("crie uma imagem de…").
func (a *App) WantsImage(text string) bool { return imageIntentRe.MatchString(text) }

type chatImageJob struct {
	conversationID string
	message        chat.Message
	model          string
	video          bool
}

// GenerateChatImage answers a chat message with an image: the request and the
// result stay in the conversation, and the image also goes to the Images page.
// reference is an optional attached image to transform (img2img).
func (a *App) GenerateChatImage(conversationID, chatModelID, text, reference string) (chat.Conversation, error) {
	return a.generateChatMedia(conversationID, chatModelID, text, reference, false)
}

// GenerateChatVideo answers with a short video; with a reference image
// attached, the video starts from it (image-to-video).
func (a *App) GenerateChatVideo(conversationID, chatModelID, text, reference string) (chat.Conversation, error) {
	return a.generateChatMedia(conversationID, chatModelID, text, reference, true)
}

func (a *App) generateChatMedia(conversationID, chatModelID, text, reference string, video bool) (chat.Conversation, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return chat.Conversation{}, errors.New(i18n.T("descreva o que você quer gerar"))
	}
	if bin, _ := a.imageBinary(); bin == "" {
		return chat.Conversation{}, errors.New(i18n.T("instale um motor de imagens em Configurações → Ferramentas para gerar imagens"))
	}
	models := a.installedFor("image", a.settings.Get().ImageModelID)
	if video {
		models = a.installedFor("video", a.settings.Get().VideoModelID)
		if reference != "" { // only image-to-video models can start from a picture
			var i2v []catalog.View
			for _, m := range models {
				if m.Video != nil && m.Video.ImageInput {
					i2v = append(i2v, m)
				}
			}
			if len(models) > 0 && len(i2v) == 0 {
				return chat.Conversation{}, errors.New(i18n.T("para animar uma imagem, baixe o Wan 2.2 5B Turbo em Descobrir → Vídeos"))
			}
			models = i2v
		}
	}
	if len(models) == 0 {
		if video {
			return chat.Conversation{}, errors.New(i18n.T("baixe um modelo de vídeo em Descobrir → Vídeos"))
		}
		return chat.Conversation{}, errors.New(i18n.T("baixe um modelo de imagem em Descobrir → Imagens"))
	}
	if a.images.Busy() != "" {
		return chat.Conversation{}, errors.New(i18n.T("já existe uma imagem sendo gerada"))
	}
	if a.chat.IsGenerating(conversationID) {
		return chat.Conversation{}, errors.New(i18n.T("aguarde a resposta atual terminar"))
	}
	img := models[0]
	store := a.chat.Store()

	var conv chat.Conversation
	var err error
	if conversationID == "" {
		if conv, err = store.Create(chatModelID, ""); err != nil {
			return conv, err
		}
		title := strings.Join(strings.Fields(text), " ")
		if utf8.RuneCountInString(title) > 48 {
			title = strings.TrimSpace(string([]rune(title)[:48])) + "…"
		}
		_ = store.Rename(conv.ID, title)
		conv.Title = title
	} else if conv, err = store.Get(conversationID); err != nil {
		return conv, err
	}

	var attachments []string
	if reference != "" {
		attachments = []string{reference}
	}
	user, err := store.AddMessage(chat.Message{ConversationID: conv.ID, Role: "user", Content: text, ModelID: chatModelID, Attachments: attachments})
	if err != nil {
		return conv, err
	}
	a.emit("chat:message", user)
	reply, err := store.AddMessage(chat.Message{ConversationID: conv.ID, Role: "assistant", ModelID: img.ID})
	if err != nil {
		return conv, err
	}
	a.emit("chat:message", reply)
	a.emit("chat:conversations")
	a.mu.Lock()
	a.imagePending[conv.ID] = true
	a.mu.Unlock()
	a.emit("chat:status", map[string]string{"conversationId": conv.ID, "status": "image"})

	go func() {
		fail := func(err error) {
			a.mu.Lock()
			delete(a.imagePending, conv.ID)
			a.mu.Unlock()
			reply.Error = err.Error()
			_ = store.UpdateMessage(reply)
			a.emit("chat:done", reply)
			a.emit("chat:status", map[string]string{"conversationId": conv.ID, "status": ""})
		}
		// Image models follow detailed English prompts much better.
		prompt := text
		if enhanced, err := a.EnhanceImagePrompt(text); err == nil {
			prompt = enhanced
		}
		spec := img.Image
		if spec == nil {
			fail(errors.New(i18n.T("modelo de imagem inválido")))
			return
		}
		_, taesd := a.catalog.ComponentPaths(img.ID)["taesd"]
		req := imagegen.Request{ModelID: img.ID, Prompt: prompt, Width: spec.Width, Height: spec.Height,
			Steps: spec.Steps, CFG: spec.CFG, Sampler: spec.Sampler, Seed: -1, FastDecode: a.settings.Get().FastDecode && taesd,
			InitImage: a.referencePath(reference)}
		if video && img.Video != nil {
			req.Video, req.Frames, req.FPS, req.Format = true, img.Video.Frames, img.Video.FPS, "webm"
		}
		job, err := a.images.Generate(req)
		if err != nil {
			fail(err)
			return
		}
		a.mu.Lock()
		a.chatImages[job] = chatImageJob{conversationID: conv.ID, message: reply, model: img.Name, video: video}
		a.mu.Unlock()
	}()
	return conv, nil
}

// chatImageFinished completes the chat message of an image job (if any).
func (a *App) chatImageFinished(jobID string, img *imagegen.Image, errMsg string) {
	a.mu.Lock()
	job, ok := a.chatImages[jobID]
	delete(a.chatImages, jobID)
	if ok {
		delete(a.imagePending, job.conversationID)
	}
	a.mu.Unlock()
	if !ok {
		return
	}
	m := job.message
	switch {
	case img != nil:
		name, err := copyAttachment(a.images.Path(img.File))
		if err != nil {
			m.Error = err.Error()
			break
		}
		m.Attachments = []string{name}
		took := humanDuration(float64(img.DurationMs) / 1000)
		if job.video {
			m.Content = i18n.Tf("Vídeo de %.1f s gerado com %s em %s.\n\n*Prompt:* %s",
				float64(img.Frames)/float64(max(img.FPS, 1)), job.model, took, img.Prompt)
		} else {
			m.Content = i18n.Tf("Imagem gerada com %s em %s.\n\n*Prompt:* %s", job.model, took, img.Prompt)
		}
	case errMsg == "":
		m.Error = i18n.T("geração cancelada")
	default:
		m.Error = errMsg
	}
	_ = a.chat.Store().UpdateMessage(m)
	a.emit("chat:done", m)
	a.emit("chat:status", map[string]string{"conversationId": job.conversationID, "status": ""})
}

// GeneratingImage reports whether a conversation is waiting for an image.
func (a *App) generatingImage(conversationID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.imagePending[conversationID]
}

// stopChatImage cancels the image being generated for a conversation.
func (a *App) stopChatImage(conversationID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, j := range a.chatImages {
		if j.conversationID == conversationID {
			a.images.Cancel()
			return true
		}
	}
	return false
}

// humanDuration formats seconds as "45 s" or "3 min 20 s".
func humanDuration(sec float64) string {
	if sec < 60 {
		return i18n.Tf("%.0f s", sec)
	}
	return i18n.Tf("%d min %02d s", int(sec)/60, int(sec)%60)
}
