package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Backend is what the chat service needs from the rest of the app.
type Backend interface {
	EnsureModel(ctx context.Context, modelID string) error
	BaseURL() string
	SupportsVision(modelID string) bool
	ContextSize() int
	Temperature() float64
	DefaultSystemPrompt() string
	AutoTitle() bool
	AttachmentPath(name string) string
	// Retrieve finds document passages relevant to query (RAG).
	Retrieve(ctx context.Context, collectionID, query string) ([]Source, error)
}

// Events lets the UI follow generation in real time.
type Events struct {
	Message       func(Message)                                                           // a message was created/replaced
	Delta         func(conversationID string, messageID int64, content, reasoning string) // accumulated text so far
	Done          func(Message)                                                           // generation finished (maybe with Error)
	Conversations func()                                                                  // list/titles changed
	Status        func(conversationID, status string)                                     // "loading", "generating", ""
}

type Service struct {
	store   *Store
	backend Backend
	events  Events

	mu      sync.Mutex
	running map[string]context.CancelFunc
	client  *http.Client
}

func NewService(store *Store, backend Backend, events Events) *Service {
	return &Service{store: store, backend: backend, events: events,
		running: map[string]context.CancelFunc{}, client: &http.Client{}}
}

func (s *Service) Store() *Store { return s.store }

func (s *Service) IsGenerating(conversationID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.running[conversationID]
	return ok
}

// Send stores the user message and starts streaming the assistant reply.
// When conversationID is empty a new conversation is created. collectionID
// selects documents to ground the answer ("" = none).
func (s *Service) Send(conversationID, modelID, collectionID, text string, attachments []string) (Conversation, error) {
	text = strings.TrimSpace(text)
	if text == "" && len(attachments) == 0 {
		return Conversation{}, errors.New("mensagem vazia")
	}
	if modelID == "" {
		return Conversation{}, errors.New("escolha um modelo para conversar")
	}
	if s.IsGenerating(conversationID) {
		return Conversation{}, errors.New("aguarde a resposta atual terminar")
	}

	var conv Conversation
	var err error
	if conversationID == "" {
		conv, err = s.store.Create(modelID, "")
		if err != nil {
			return conv, err
		}
	} else if conv, err = s.store.Get(conversationID); err != nil {
		return conv, err
	}
	if conv.ModelID != modelID {
		_ = s.store.SetModel(conv.ID, modelID)
		conv.ModelID = modelID
	}
	if conv.CollectionID != collectionID {
		_ = s.store.SetCollection(conv.ID, collectionID)
		conv.CollectionID = collectionID
	}

	user, err := s.store.AddMessage(Message{ConversationID: conv.ID, Role: "user", Content: text,
		Attachments: attachments, ModelID: modelID})
	if err != nil {
		return conv, err
	}
	s.emitMessage(user)
	s.emitConversations()
	return conv, s.generate(conv)
}

// Regenerate drops the last assistant reply and asks again.
func (s *Service) Regenerate(conversationID string) error {
	msgs, err := s.store.Messages(conversationID)
	if err != nil {
		return err
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			break
		}
		if msgs[i].Role == "assistant" {
			if err := s.store.DeleteFrom(conversationID, msgs[i].ID); err != nil {
				return err
			}
			break
		}
	}
	conv, err := s.store.Get(conversationID)
	if err != nil {
		return err
	}
	return s.generate(conv)
}

// Edit replaces a user message (and drops everything after it), then regenerates.
func (s *Service) Edit(conversationID string, messageID int64, text string) error {
	msgs, err := s.store.Messages(conversationID)
	if err != nil {
		return err
	}
	var original *Message
	for i := range msgs {
		if msgs[i].ID == messageID {
			original = &msgs[i]
		}
	}
	if original == nil || original.Role != "user" {
		return errors.New("mensagem não encontrada")
	}
	if err := s.store.DeleteFrom(conversationID, messageID); err != nil {
		return err
	}
	conv, err := s.store.Get(conversationID)
	if err != nil {
		return err
	}
	user, err := s.store.AddMessage(Message{ConversationID: conversationID, Role: "user", Content: strings.TrimSpace(text),
		Attachments: original.Attachments, ModelID: conv.ModelID})
	if err != nil {
		return err
	}
	s.emitMessage(user)
	return s.generate(conv)
}

func (s *Service) Stop(conversationID string) {
	s.mu.Lock()
	cancel := s.running[conversationID]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Service) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, cancel := range s.running {
		cancel()
	}
}

func (s *Service) generate(conv Conversation) error {
	assistant, err := s.store.AddMessage(Message{ConversationID: conv.ID, Role: "assistant", ModelID: conv.ModelID})
	if err != nil {
		return err
	}
	s.emitMessage(assistant)

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.running[conv.ID] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.running, conv.ID)
			s.mu.Unlock()
			cancel()
			s.status(conv.ID, "")
		}()

		s.status(conv.ID, "loading")
		if err := s.backend.EnsureModel(ctx, conv.ModelID); err != nil {
			assistant.Error = "Não foi possível carregar o modelo: " + err.Error()
			s.finish(assistant)
			return
		}
		var sources []Source
		if conv.CollectionID != "" {
			s.status(conv.ID, "searching")
			var err error
			sources, err = s.retrieve(ctx, conv)
			if err != nil {
				if ctx.Err() == nil {
					assistant.Error = "Não foi possível buscar nos documentos: " + err.Error()
				}
				s.finish(assistant)
				return
			}
			assistant.Sources = sources
			s.emitMessage(assistant)
		}
		s.status(conv.ID, "generating")

		messages, err := s.buildHistory(conv, sources)
		if err != nil {
			assistant.Error = err.Error()
			s.finish(assistant)
			return
		}
		err = s.stream(ctx, conv, &assistant, messages)
		switch {
		case errors.Is(err, context.Canceled):
			// Stopped by the user: keep the partial answer.
		case err != nil:
			assistant.Error = err.Error()
		}
		s.finish(assistant)
		if err == nil || errors.Is(err, context.Canceled) {
			s.maybeTitle(conv.ID)
		}
	}()
	return nil
}

func (s *Service) finish(m Message) {
	_ = s.store.UpdateMessage(m)
	if s.events.Done != nil {
		s.events.Done(m)
	}
	s.emitConversations()
}

func (s *Service) emitMessage(m Message) {
	if s.events.Message != nil {
		s.events.Message(m)
	}
}

func (s *Service) emitConversations() {
	if s.events.Conversations != nil {
		s.events.Conversations()
	}
}

func (s *Service) status(id, st string) {
	if s.events.Status != nil {
		s.events.Status(id, st)
	}
}

type apiMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func (s *Service) imageDataURL(name string) (string, error) {
	path := s.backend.AttachmentPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mt := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if mt == "" {
		mt = "image/png"
	}
	return "data:" + mt + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// buildHistory converts stored messages to the API format, keeping the most
// recent ones that fit in the context window (~3 chars per token).
// retrieve searches the conversation documents with the last user message.
func (s *Service) retrieve(ctx context.Context, conv Conversation) ([]Source, error) {
	msgs, err := s.store.Messages(conv.ID)
	if err != nil {
		return nil, err
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" && strings.TrimSpace(msgs[i].Content) != "" {
			return s.backend.Retrieve(ctx, conv.CollectionID, msgs[i].Content)
		}
	}
	return nil, nil
}

// ragPrompt tells the model how to use the retrieved passages.
func ragPrompt(sources []Source) string {
	if len(sources) == 0 {
		return "\n\nO usuário anexou documentos, mas nenhum trecho relevante foi encontrado para esta pergunta. Diga isso se a resposta depender deles."
	}
	var b strings.Builder
	b.WriteString("\n\nUse os trechos abaixo, extraídos dos documentos do usuário, para responder. ")
	b.WriteString("Cite as fontes com o número entre colchetes, por exemplo [1]. ")
	b.WriteString("Se os trechos não tiverem a resposta, diga que não encontrou nos documentos.\n")
	for _, src := range sources {
		fmt.Fprintf(&b, "\n[%d] %s (trecho %d):\n%s\n", src.N, src.Document, src.Chunk+1, src.Text)
	}
	return b.String()
}

func (s *Service) buildHistory(conv Conversation, sources []Source) ([]apiMessage, error) {
	msgs, err := s.store.Messages(conv.ID)
	if err != nil {
		return nil, err
	}
	vision := s.backend.SupportsVision(conv.ModelID)
	budget := s.backend.ContextSize() * 3 * 3 / 4 // leave room for the answer
	rag := ""
	if conv.CollectionID != "" {
		rag = ragPrompt(sources)
		budget -= utf8.RuneCountInString(rag)
	}

	var out []apiMessage
	used := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role == "assistant" && (m.Content == "" || m.Error != "") {
			continue
		}
		used += utf8.RuneCountInString(m.Content)
		if used > budget && len(out) > 0 {
			break
		}
		var content any = m.Content
		if m.Role == "user" && vision && len(m.Attachments) > 0 {
			parts := []map[string]any{}
			for _, a := range m.Attachments {
				url, err := s.imageDataURL(a)
				if err != nil {
					continue
				}
				used += 1000 // images cost hundreds of tokens
				parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]string{"url": url}})
			}
			parts = append(parts, map[string]any{"type": "text", "text": m.Content})
			content = parts
		}
		out = append(out, apiMessage{Role: m.Role, Content: content})
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}

	system := conv.SystemPrompt
	if system == "" {
		system = s.backend.DefaultSystemPrompt()
	}
	system += rag
	if system != "" {
		out = append([]apiMessage{{Role: "system", Content: system}}, out...)
	}
	return out, nil
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Timings *struct {
		PredictedN         int     `json:"predicted_n"`
		PredictedPerSecond float64 `json:"predicted_per_second"`
	} `json:"timings"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (s *Service) stream(ctx context.Context, conv Conversation, msg *Message, messages []apiMessage) error {
	body, _ := json.Marshal(map[string]any{
		"model":       conv.ModelID,
		"messages":    messages,
		"stream":      true,
		"temperature": s.backend.Temperature(),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.backend.BaseURL()+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("falha ao conectar ao llamafile: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var e streamChunk
		if json.Unmarshal(data, &e) == nil && e.Error != nil {
			return fmt.Errorf("llamafile: %s", e.Error.Message)
		}
		return fmt.Errorf("llamafile respondeu HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	started := time.Now()
	tokens := 0
	lastSave := time.Now()
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				break
			}
			var chunk streamChunk
			if json.Unmarshal([]byte(payload), &chunk) == nil {
				if chunk.Error != nil {
					return fmt.Errorf("llamafile: %s", chunk.Error.Message)
				}
				for _, c := range chunk.Choices {
					if c.Delta.Content != "" || c.Delta.ReasoningContent != "" {
						tokens++
						msg.Content += c.Delta.Content
						msg.Reasoning += c.Delta.ReasoningContent
						if s.events.Delta != nil {
							s.events.Delta(conv.ID, msg.ID, msg.Content, msg.Reasoning)
						}
					}
				}
				if chunk.Timings != nil && chunk.Timings.PredictedPerSecond > 0 {
					msg.TPS = chunk.Timings.PredictedPerSecond
					msg.Tokens = chunk.Timings.PredictedN
				}
			}
			if time.Since(lastSave) > 2*time.Second { // survive crashes mid-answer
				_ = s.store.UpdateMessage(*msg)
				lastSave = time.Now()
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
	}
	if msg.TPS == 0 && tokens > 0 {
		msg.Tokens = tokens
		msg.TPS = float64(tokens) / time.Since(started).Seconds()
	}
	return nil
}

// maybeTitle names a new conversation after its first exchange, like other chat apps.
func (s *Service) maybeTitle(conversationID string) {
	conv, err := s.store.Get(conversationID)
	if err != nil || conv.Title != DefaultTitle {
		return
	}
	msgs, err := s.store.Messages(conversationID)
	if err != nil || len(msgs) == 0 {
		return
	}
	var firstUser, firstAnswer string
	for _, m := range msgs {
		if m.Role == "user" && firstUser == "" {
			firstUser = m.Content
		}
		if m.Role == "assistant" && firstAnswer == "" {
			firstAnswer = m.Content
		}
	}
	title := fallbackTitle(firstUser)
	if s.backend.AutoTitle() && firstAnswer != "" {
		if t := s.askTitle(firstUser, firstAnswer, conv.ModelID); t != "" {
			title = t
		}
	}
	_ = s.store.Rename(conversationID, title)
	s.emitConversations()
}

func fallbackTitle(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) > 48 {
		r := []rune(text)
		text = strings.TrimSpace(string(r[:48])) + "…"
	}
	if text == "" {
		return DefaultTitle
	}
	return text
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func (s *Service) askTitle(question, answer, modelID string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	body, _ := json.Marshal(map[string]any{
		"model": modelID,
		"messages": []apiMessage{
			{Role: "system", Content: "Crie um título curto (no máximo 6 palavras) para a conversa abaixo, no idioma da pergunta. Responda só com o título, sem aspas nem pontuação final."},
			{Role: "user", Content: "Pergunta: " + truncate(question, 600) + "\n\nResposta: " + truncate(answer, 600)},
		},
		"max_tokens":           24,
		"temperature":          0.2,
		"stream":               false,
		"chat_template_kwargs": map[string]any{"enable_thinking": false},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.backend.BaseURL()+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil || len(out.Choices) == 0 {
		return ""
	}
	t := out.Choices[0].Message.Content
	if i := strings.Index(t, "</think>"); i >= 0 {
		t = t[i+len("</think>"):]
	}
	t = strings.TrimSpace(strings.Split(strings.TrimSpace(t), "\n")[0])
	t = strings.Trim(t, "\"'*#.` ")
	t = strings.TrimPrefix(t, "Título: ")
	if t == "" || utf8.RuneCountInString(t) > 60 {
		return ""
	}
	return t
}
