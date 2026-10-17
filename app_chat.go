package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/devalexandre/llamafile-ui/internal/chat"
	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/settings"
)

// ------------------------------------------------- chat.Backend interface

func (a *App) EnsureModel(ctx context.Context, id string) error {
	if a.catalog.ModelPath(id) == "" {
		return fmt.Errorf("o modelo não está instalado")
	}
	_ = a.updateSettings(func(s *settings.Settings) { s.LastModelID = id })
	return a.runtime.EnsureModel(ctx, id)
}
func (a *App) BaseURL() string               { return a.runtime.Status().BaseURL }
func (a *App) SupportsVision(id string) bool { return a.catalog.MMProjPath(id) != "" }
func (a *App) ContextSize() int              { return a.settings.Get().ContextSize }
func (a *App) Temperature() float64          { return a.settings.Get().Temperature }
func (a *App) DefaultSystemPrompt() string   { return a.settings.Get().SystemPrompt }
func (a *App) AutoTitle() bool               { return a.settings.Get().AutoTitle }
func (a *App) AttachmentPath(name string) string {
	return filepath.Join(paths.AttachmentsDir(), filepath.Base(name))
}

// Retrieve searches a document collection for the chat (RAG).
func (a *App) Retrieve(ctx context.Context, collectionID, query string) ([]chat.Source, error) {
	hits, err := a.docs.Search(ctx, collectionID, query, 5)
	if err != nil {
		return nil, err
	}
	sources := []chat.Source{}
	for _, h := range hits {
		if h.Score < 0.2 && len(sources) > 0 {
			break // keep only reasonably related passages
		}
		sources = append(sources, chat.Source{N: len(sources) + 1, Document: h.Document, Chunk: h.Chunk, Text: h.Text, Score: h.Score})
	}
	return sources, nil
}

// ------------------------------------------------------------------ chat

type ConversationDetail struct {
	Conversation chat.Conversation `json:"conversation"`
	Messages     []chat.Message    `json:"messages"`
	Generating   bool              `json:"generating"`
}

func (a *App) ListConversations(search string) ([]chat.Conversation, error) {
	return a.chat.Store().List(search)
}

func (a *App) GetConversation(id string) (ConversationDetail, error) {
	c, err := a.chat.Store().Get(id)
	if err != nil {
		return ConversationDetail{}, err
	}
	msgs, err := a.chat.Store().Messages(id)
	return ConversationDetail{Conversation: c, Messages: msgs, Generating: a.chat.IsGenerating(id)}, err
}

func (a *App) SendMessage(conversationID, modelID, collectionID, text string, attachments []string) (chat.Conversation, error) {
	return a.chat.Send(conversationID, modelID, collectionID, text, attachments)
}

func (a *App) StopGeneration(conversationID string)   { a.chat.Stop(conversationID) }
func (a *App) Regenerate(conversationID string) error { return a.chat.Regenerate(conversationID) }
func (a *App) EditMessage(conversationID string, messageID int64, text string) error {
	return a.chat.Edit(conversationID, messageID, text)
}

func (a *App) RenameConversation(id, title string) error {
	err := a.chat.Store().Rename(id, title)
	a.emit("chat:conversations")
	return err
}

func (a *App) PinConversation(id string, pinned bool) error {
	err := a.chat.Store().SetPinned(id, pinned)
	a.emit("chat:conversations")
	return err
}

func (a *App) SetConversationModel(id, modelID string) error {
	return a.chat.Store().SetModel(id, modelID)
}

func (a *App) SetConversationCollection(id, collectionID string) error {
	return a.chat.Store().SetCollection(id, collectionID)
}

func (a *App) SetConversationSystemPrompt(id, prompt string) error {
	return a.chat.Store().SetSystemPrompt(id, prompt)
}

func (a *App) DeleteConversation(id string) error {
	a.chat.Stop(id)
	err := a.chat.Store().Delete(id)
	a.emit("chat:conversations")
	return err
}

func (a *App) DeleteAllConversations() error {
	a.chat.StopAll()
	err := a.chat.Store().DeleteAll()
	a.emit("chat:conversations")
	return err
}

func safeFileName(name string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, name)
}

// saveAs asks where to save and writes content there. Returns "" if cancelled.
func (a *App) saveAs(title, defaultName string, content []byte) (string, error) {
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{Title: title, DefaultFilename: safeFileName(defaultName)})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, content, 0o644)
}

// ExportConversation asks where to save and writes Markdown or JSON.
func (a *App) ExportConversation(id, format string) (string, error) {
	content, err := a.chat.Store().Export(id, format)
	if err != nil {
		return "", err
	}
	c, _ := a.chat.Store().Get(id)
	ext := ".md"
	if format == "json" {
		ext = ".json"
	}
	return a.saveAs("Exportar conversa", c.Title+ext, []byte(content))
}

// PickImages lets the user attach images; returns attachment names.
func (a *App) PickImages() ([]string, error) {
	files, err := wruntime.OpenMultipleFilesDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:   "Anexar imagens",
		Filters: []wruntime.FileFilter{{DisplayName: "Imagens", Pattern: "*.png;*.jpg;*.jpeg;*.webp;*.gif"}},
	})
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, f := range files {
		name, err := copyAttachment(f)
		if err != nil {
			return names, err
		}
		names = append(names, name)
	}
	return names, nil
}

func copyAttachment(src string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	name := uuid.NewString() + strings.ToLower(filepath.Ext(src))
	out, err := os.Create(filepath.Join(paths.AttachmentsDir(), name))
	if err != nil {
		return "", err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return name, err
}

// SaveAttachment stores a pasted/dropped image (base64) and returns its name.
func (a *App) SaveAttachment(data, ext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}
	ext = "." + strings.TrimPrefix(strings.ToLower(ext), ".")
	name := uuid.NewString() + ext
	return name, os.WriteFile(filepath.Join(paths.AttachmentsDir(), name), raw, 0o644)
}

// complete runs a single non-streaming chat completion on the chat server.
func (a *App) complete(ctx context.Context, modelID, system, user string, maxTokens int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model": modelID,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"max_tokens":           maxTokens,
		"temperature":          0.4,
		"chat_template_kwargs": map[string]any{"enable_thinking": false},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.runtime.Status().BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || len(out.Choices) == 0 {
		return "", fmt.Errorf("resposta inválida do llamafile")
	}
	text := out.Choices[0].Message.Content
	if i := strings.Index(text, "</think>"); i >= 0 {
		text = text[i+len("</think>"):]
	}
	return text, nil
}
