package main

import (
	"context"
	"errors"
	"strings"

	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"github.com/devalexandre/llamafile-ui/internal/rag"
	"github.com/devalexandre/llamafile-ui/internal/settings"
)

// -------------------------------------------------------------- documents

func (a *App) embedModel() (string, error) {
	list := a.installedFor("embedding", a.settings.Get().EmbedModelID)
	if len(list) == 0 {
		return "", errors.New(i18n.T("baixe um modelo de embeddings em Descobrir → Embeddings"))
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
	files, err := a.openFiles(i18n.T("Adicionar documentos"),
		fileFilter{i18n.T("Documentos"), "*" + strings.Join(rag.SupportedExtensions(), ";*")})
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
