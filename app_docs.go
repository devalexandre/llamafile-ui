package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/devalexandre/llamafile-ui/internal/rag"
	"github.com/devalexandre/llamafile-ui/internal/settings"
)

// ------------------------------------------------- rag.Embedder interface

// Embed runs texts through the embeddings server, starting it if needed.
func (a *App) Embed(ctx context.Context, modelID string, texts []string) ([][]float32, error) {
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

func (a *App) Prefixes(modelID string) (doc, query string) {
	if m := a.catalog.Find(modelID); m != nil && m.Embed != nil {
		return m.Embed.DocPrefix, m.Embed.QueryPrefix
	}
	return "", ""
}

// -------------------------------------------------------------- documents

func (a *App) embedModel() (string, error) {
	list := a.installedFor("embedding", a.settings.Get().EmbedModelID)
	if len(list) == 0 {
		return "", errors.New("baixe um modelo de embeddings em Descobrir → Embeddings")
	}
	return list[0].ID, nil
}

func (a *App) SetEmbedModel(id string) error {
	return a.updateSettings(func(s *settings.Settings) { s.EmbedModelID = id })
}

func (a *App) ListCollections() ([]rag.Collection, error) { return a.docs.Collections() }

func (a *App) CreateCollection(name string) (rag.Collection, error) {
	model, err := a.embedModel()
	if err != nil {
		return rag.Collection{}, err
	}
	return a.docs.CreateCollection(name, model)
}

func (a *App) RenameCollection(id, name string) error { return a.docs.RenameCollection(id, name) }
func (a *App) DeleteCollection(id string) error       { return a.docs.DeleteCollection(id) }
func (a *App) ReindexCollection(id string) error      { return a.docs.Reindex(id) }

func (a *App) SetCollectionModel(id, model string) error { return a.docs.SetEmbedModel(id, model) }

func (a *App) ListDocuments(collectionID string) ([]rag.Document, error) {
	return a.docs.Documents(collectionID)
}

// AddDocuments opens a file dialog and indexes the chosen files.
func (a *App) AddDocuments(collectionID string) ([]rag.Document, error) {
	files, err := wruntime.OpenMultipleFilesDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Adicionar documentos",
		Filters: []wruntime.FileFilter{
			{DisplayName: "Documentos", Pattern: "*" + strings.Join(rag.SupportedExtensions(), ";*")},
		},
	})
	if err != nil || len(files) == 0 {
		return []rag.Document{}, err
	}
	return a.docs.AddFiles(collectionID, files)
}

// AddDocumentPaths indexes files dropped on the window.
func (a *App) AddDocumentPaths(collectionID string, files []string) ([]rag.Document, error) {
	return a.docs.AddFiles(collectionID, files)
}

func (a *App) RemoveDocument(id int64) error { return a.docs.RemoveDocument(id) }

func (a *App) SearchCollection(collectionID, query string) ([]rag.Hit, error) {
	return a.docs.Search(context.Background(), collectionID, query, 6)
}
