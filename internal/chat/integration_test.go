//go:build integration

package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/runtime"
	"github.com/devalexandre/llamafile-ui/internal/settings"
)

// Run with: go test -tags integration ./internal/chat -run Integration -v
// Needs a llamafile binary in the app bin dir and qwen2.5-0.5b in the models dir.
type resolver struct{ path string }

func (r resolver) ModelPath(string) string   { return r.path }
func (r resolver) MMProjPath(string) string  { return "" }
func (r resolver) DisplayName(string) string { return "Qwen 0.5B" }

type backend struct {
	rt  *runtime.Manager
	cfg settings.Settings
}

func (b backend) EnsureModel(ctx context.Context, id string) error           { return b.rt.EnsureModel(ctx, id) }
func (b backend) BaseURL() string                                            { return b.rt.Status().BaseURL }
func (b backend) SupportsVision(string) bool                                 { return false }
func (b backend) ContextSize() int                                           { return 2048 }
func (b backend) Temperature() float64                                       { return 0.2 }
func (b backend) DefaultSystemPrompt() string                                { return "Responda em uma frase." }
func (b backend) AutoTitle() bool                                            { return true }
func (b backend) AttachmentPath(n string) string                             { return n }
func (b backend) Retrieve(context.Context, string, string) ([]Source, error) { return nil, nil }

func TestIntegrationChat(t *testing.T) {
	model := filepath.Join(paths.DefaultModelsDir(), "qwen2.5-0.5b-instruct-q4_k_m.gguf")
	if _, err := os.Stat(model); err != nil {
		t.Skip("model not downloaded")
	}
	bins, _ := filepath.Glob(filepath.Join(paths.BinDir(), "llamafile-*"))
	if len(bins) == 0 {
		t.Skip("llamafile not installed")
	}
	// Isolate settings from the real user profile.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	st := settings.Load()
	if _, err := st.Update(func(s *settings.Settings) {
		s.Port, s.ContextSize, s.LlamafilePath = 18686, 2048, bins[0]
	}); err != nil {
		t.Fatal(err)
	}

	rt := runtime.New(st, resolver{model}, runtime.ChatRole)
	if rt.Status().State == runtime.StateMissing {
		t.Skip("llamafile not installed")
	}
	defer rt.Stop()

	store, err := Open(filepath.Join(t.TempDir(), "chats.db"))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan Message, 1)
	deltas := 0
	svc := NewService(store, backend{rt: rt}, Events{
		Delta: func(string, int64, string, string) { deltas++ },
		Done:  func(m Message) { done <- m },
	})
	conv, err := svc.Send("", "qwen2.5-0.5b", "", "Qual a capital do Brasil?", nil)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-done:
		if m.Error != "" {
			t.Fatalf("error: %s\nlogs:\n%v", m.Error, rt.Logs()[max(0, len(rt.Logs())-15):])
		}
		t.Logf("answer (%d deltas, %.1f tok/s): %s", deltas, m.TPS, m.Content)
	case <-time.After(3 * time.Minute):
		t.Fatalf("timeout; logs: %v", rt.Logs())
	}
	time.Sleep(3 * time.Second) // let auto-title finish
	c, _ := store.Get(conv.ID)
	msgs, _ := store.Messages(conv.ID)
	t.Logf("title=%q messages=%d", c.Title, len(msgs))
	if len(msgs) != 2 {
		t.Fatalf("expected 2 stored messages, got %d", len(msgs))
	}
}
