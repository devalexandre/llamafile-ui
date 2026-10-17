//go:build integration

package rag

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/runtime"
	"github.com/devalexandre/llamafile-ui/internal/settings"
)

type resolver struct{ path string }

func (r resolver) ModelPath(string) string   { return r.path }
func (r resolver) MMProjPath(string) string  { return "" }
func (r resolver) DisplayName(string) string { return "nomic" }

type embedder struct{ rt *runtime.Manager }

func (e embedder) Prefixes(string) (string, string) { return "search_document: ", "search_query: " }
func (e embedder) Embed(ctx context.Context, model string, texts []string) ([][]float32, error) {
	if err := e.rt.EnsureModel(ctx, model); err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]any{"input": texts})
	resp, err := http.Post(e.rt.Status().BaseURL+"/v1/embeddings", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	vecs := make([][]float32, len(texts))
	for _, d := range out.Data {
		vecs[d.Index] = d.Embedding
	}
	return vecs, nil
}

// go test -tags integration ./internal/rag -v
func TestIntegrationRAG(t *testing.T) {
	model := filepath.Join(paths.DefaultModelsDir(), "nomic-embed-text-v1.5.Q8_0.gguf")
	bins, _ := filepath.Glob(filepath.Join(paths.BinDir(), "llamafile-*"))
	if _, err := os.Stat(model); err != nil || len(bins) == 0 {
		t.Skip("needs nomic model and llamafile")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	st := settings.Load()
	st.Update(func(s *settings.Settings) { s.Port = 18690; s.LlamafilePath = bins[0] })
	rt := runtime.New(st, resolver{model}, runtime.EmbedRole)
	defer rt.Stop()

	db, _ := sql.Open("sqlite", filepath.Join(t.TempDir(), "rag.db"))
	db.SetMaxOpenConns(1)
	db.Exec("PRAGMA foreign_keys = ON")
	ready := make(chan Document, 4)
	s, err := Open(db, embedder{rt}, Events{Document: func(d Document) {
		if d.Status == "ready" || d.Status == "error" {
			ready <- d
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "gatos.md"), []byte("# Gatos\n\nGatos dormem cerca de 15 horas por dia e são caçadores noturnos.\n\nO ronronar dos gatos ajuda na cicatrização óssea."), 0o644)
	os.WriteFile(filepath.Join(dir, "foguetes.txt"), []byte("O foguete Saturn V levou astronautas à Lua em 1969. Ele tinha três estágios e usava querosene e hidrogênio líquido."), 0o644)
	c, _ := s.CreateCollection("teste", "nomic")
	if _, err := s.AddFiles(c.ID, []string{filepath.Join(dir, "gatos.md"), filepath.Join(dir, "foguetes.txt")}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		select {
		case d := <-ready:
			t.Logf("%s: %s %s chunks=%d", d.Name, d.Status, d.Error, d.Chunks)
		case <-time.After(3 * time.Minute):
			t.Fatal("timeout indexing")
		}
	}
	hits, err := s.Search(context.Background(), c.ID, "quantas horas um gato dorme?", 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		t.Logf("%.3f %s: %.60s", h.Score, h.Document, h.Text)
	}
	if len(hits) == 0 || hits[0].Document != "gatos.md" {
		t.Fatalf("expected gatos.md first, got %+v", hits)
	}
}
