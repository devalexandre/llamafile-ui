package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/devalexandre/llamafile-ui/internal/catalog"
	"github.com/devalexandre/llamafile-ui/internal/chat"
	"github.com/devalexandre/llamafile-ui/internal/imagegen"
	"github.com/devalexandre/llamafile-ui/internal/paths"
	"github.com/devalexandre/llamafile-ui/internal/settings"
)

// backend implements the interfaces used by internal packages (chat, rag,
// imagegen). It is not registered as a Wails service, so none of these
// methods are exposed to the frontend.
type backend struct{ *App }

// ------------------------------------------------- chat.Backend interface

func (a *backend) EnsureModel(ctx context.Context, id string) error {
	if a.catalog.ModelPath(id) == "" {
		return fmt.Errorf("o modelo não está instalado")
	}
	_ = a.updateSettings(func(s *settings.Settings) { s.LastModelID = id })
	return a.runtime.EnsureModel(ctx, id)
}
func (a *backend) BaseURL() string               { return a.runtime.Status().BaseURL }
func (a *backend) SupportsVision(id string) bool { return a.catalog.MMProjPath(id) != "" }
func (a *backend) ContextSize() int              { return a.settings.Get().ContextSize }
func (a *backend) Temperature() float64          { return a.settings.Get().Temperature }
func (a *backend) DefaultSystemPrompt() string   { return a.settings.Get().SystemPrompt }
func (a *backend) AutoTitle() bool               { return a.settings.Get().AutoTitle }
func (a *backend) AttachmentPath(name string) string {
	return filepath.Join(paths.AttachmentsDir(), filepath.Base(name))
}

// Retrieve searches a document collection for the chat (RAG).
func (a *backend) Retrieve(ctx context.Context, collectionID, query string) ([]chat.Source, error) {
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

// ------------------------------------------------- rag.Embedder interface

// Embed runs texts through the embeddings server, starting it if needed.
func (a *backend) Embed(ctx context.Context, modelID string, texts []string) ([][]float32, error) {
	if a.catalog.ModelPath(modelID) == "" {
		return nil, fmt.Errorf("o modelo de embeddings %q não está instalado", a.catalog.DisplayName(modelID))
	}
	if err := a.embedder.EnsureModel(ctx, modelID); err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]any{"model": modelID, "input": texts})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.embedder.Status().BaseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("servidor de embeddings: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	vecs := make([][]float32, len(texts))
	for _, d := range out.Data {
		if d.Index >= 0 && d.Index < len(vecs) {
			vecs[d.Index] = d.Embedding
		}
	}
	for _, v := range vecs {
		if len(v) == 0 {
			return nil, errors.New("o servidor não retornou todos os embeddings")
		}
	}
	return vecs, nil
}

func (a *backend) Prefixes(modelID string) (doc, query string) {
	if m := a.catalog.Find(modelID); m != nil && m.Embed != nil {
		return m.Embed.DocPrefix, m.Embed.QueryPrefix
	}
	return "", ""
}

// ------------------------------------------------ imagegen.Backend interface

func (a *backend) Binary() string {
	bin, _ := a.imageBinary()
	return bin
}

func (a *backend) Model(id string) (imagegen.ModelSpec, error) {
	m := a.catalog.Find(id)
	if m == nil || m.Image == nil {
		return imagegen.ModelSpec{}, errors.New("escolha um modelo de imagem")
	}
	main := a.catalog.ModelPath(id)
	if main == "" {
		return imagegen.ModelSpec{}, fmt.Errorf("%s não está instalado", m.Name)
	}
	return imagegen.ModelSpec{Name: m.Name, MainFlag: m.Image.MainFlag, MainPath: main,
		Components: a.catalog.ComponentPaths(id), Args: m.Image.Args}, nil
}

func (a *backend) Estimate(req imagegen.Request) float64 {
	m := a.catalog.Find(req.ModelID)
	if m == nil {
		return 0
	}
	f, ok := a.catalog.InstalledFile(m)
	if !ok && len(m.Files) > 0 {
		f = m.Files[0]
	}
	if m.Video != nil {
		frames := req.Frames
		if frames <= 1 {
			frames = m.Video.Frames
		}
		return a.calibratedImageSeconds(m, f, catalog.EstimateVideoSeconds(m, f, a.fitHardware(), req.Width, req.Height, frames, req.Steps))
	}
	_, hasTAESD := a.catalog.ComponentPaths(req.ModelID)["taesd"]
	if req.InitImage != "" {
		// img2img only runs steps × strength denoising steps.
		r := req
		r.InitImage = a.referencePath(r.InitImage)
		if imagegen.PrepareImg2Img(&r) == nil {
			req.Width, req.Height, req.Steps = r.Width, r.Height, max(1, int(float64(r.Steps)*r.Strength+0.5))
		}
	}
	return a.calibratedImageSeconds(m, f, catalog.EstimateImageSeconds(m, f, a.fitHardware(), req.Width, req.Height, req.Steps, req.FastDecode && hasTAESD))
}
