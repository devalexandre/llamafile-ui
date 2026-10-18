// Package rag indexes the user's documents with an embedding model and
// retrieves the most relevant passages for a question (retrieval-augmented
// generation).
package rag

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"github.com/google/uuid"
)

const (
	chunkSize    = 1200
	chunkOverlap = 200
	batchSize    = 8
)

// Embedder turns texts into vectors with an embedding model.
type Embedder interface {
	Embed(ctx context.Context, modelID string, texts []string) ([][]float32, error)
	// Prefixes returns the instruction prefixes the model expects (may be empty).
	Prefixes(modelID string) (doc, query string)
}

type Collection struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	EmbedModel string `json:"embedModel"`
	Documents  int    `json:"documents"`
	Chunks     int    `json:"chunks"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}

type Document struct {
	ID           int64   `json:"id"`
	CollectionID string  `json:"collectionId"`
	Name         string  `json:"name"`
	Path         string  `json:"path"`
	Size         int64   `json:"size"`
	Chars        int     `json:"chars"`
	Chunks       int     `json:"chunks"`
	Status       string  `json:"status"` // queued, indexing, ready, error
	Error        string  `json:"error"`
	Progress     float64 `json:"progress"`
	CreatedAt    int64   `json:"createdAt"`
}

type Hit struct {
	DocumentID int64   `json:"documentId"`
	Document   string  `json:"document"`
	Chunk      int     `json:"chunk"`
	Text       string  `json:"text"`
	Score      float64 `json:"score"`
}

type Events struct {
	Document func(Document) // status/progress of a document changed
	Changed  func()         // collections or documents changed
}

type vector struct {
	chunkID int64
	docID   int64
	vec     []float32
}

type Store struct {
	db       *sql.DB
	embedder Embedder
	events   Events

	mu     sync.Mutex
	cache  map[string][]vector // collection id → vectors
	queue  chan int64
	cancel map[int64]context.CancelFunc
}

const schema = `
CREATE TABLE IF NOT EXISTS rag_collections (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    embed_model TEXT NOT NULL,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS rag_documents (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    collection_id TEXT NOT NULL REFERENCES rag_collections(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    path          TEXT NOT NULL,
    size          INTEGER NOT NULL DEFAULT 0,
    chars         INTEGER NOT NULL DEFAULT 0,
    chunks        INTEGER NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT 'queued',
    error         TEXT NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS rag_chunks (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id   INTEGER NOT NULL REFERENCES rag_documents(id) ON DELETE CASCADE,
    collection_id TEXT NOT NULL,
    idx           INTEGER NOT NULL,
    text          TEXT NOT NULL,
    embedding     BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_rag_chunks_collection ON rag_chunks(collection_id);
CREATE INDEX IF NOT EXISTS idx_rag_documents_collection ON rag_documents(collection_id);
`

func Open(db *sql.DB, embedder Embedder, events Events) (*Store, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("rag db: %w", err)
	}
	s := &Store{db: db, embedder: embedder, events: events, cache: map[string][]vector{},
		queue: make(chan int64, 1024), cancel: map[int64]context.CancelFunc{}}
	// Documents interrupted by a previous shutdown go back to the queue.
	_, _ = db.Exec(`UPDATE rag_documents SET status = 'queued' WHERE status = 'indexing'`)
	go s.worker()
	if ids, err := s.queuedIDs(); err == nil {
		for _, id := range ids {
			s.queue <- id
		}
	}
	return s, nil
}

func now() int64 { return time.Now().UnixMilli() }

func (s *Store) changed() {
	if s.events.Changed != nil {
		s.events.Changed()
	}
}

func (s *Store) emitDoc(d Document) {
	if s.events.Document != nil {
		s.events.Document(d)
	}
}

// ---------------------------------------------------------------- collections

func (s *Store) CreateCollection(name, embedModel string) (Collection, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = i18n.T("Nova coleção")
	}
	if embedModel == "" {
		return Collection{}, errors.New(i18n.T("instale um modelo de embeddings primeiro"))
	}
	t := now()
	c := Collection{ID: uuid.NewString(), Name: name, EmbedModel: embedModel, CreatedAt: t, UpdatedAt: t}
	_, err := s.db.Exec(`INSERT INTO rag_collections (id, name, embed_model, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.EmbedModel, t, t)
	s.changed()
	return c, err
}

func (s *Store) Collections() ([]Collection, error) {
	rows, err := s.db.Query(`
SELECT c.id, c.name, c.embed_model, c.created_at, c.updated_at,
       (SELECT COUNT(*) FROM rag_documents d WHERE d.collection_id = c.id),
       (SELECT COUNT(*) FROM rag_chunks k WHERE k.collection_id = c.id)
FROM rag_collections c ORDER BY c.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Collection{}
	for rows.Next() {
		var c Collection
		if err := rows.Scan(&c.ID, &c.Name, &c.EmbedModel, &c.CreatedAt, &c.UpdatedAt, &c.Documents, &c.Chunks); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) Collection(id string) (Collection, error) {
	var c Collection
	err := s.db.QueryRow(`SELECT id, name, embed_model, created_at, updated_at FROM rag_collections WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &c.EmbedModel, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *Store) RenameCollection(id, name string) error {
	_, err := s.db.Exec(`UPDATE rag_collections SET name = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(name), now(), id)
	s.changed()
	return err
}

func (s *Store) DeleteCollection(id string) error {
	docs, _ := s.Documents(id)
	for _, d := range docs {
		s.stopIndexing(d.ID)
	}
	_, err := s.db.Exec(`DELETE FROM rag_collections WHERE id = ?`, id)
	s.invalidate(id)
	s.changed()
	return err
}

// SetEmbedModel switches the model of a collection and re-indexes everything.
func (s *Store) SetEmbedModel(id, model string) error {
	if _, err := s.db.Exec(`UPDATE rag_collections SET embed_model = ?, updated_at = ? WHERE id = ?`, model, now(), id); err != nil {
		return err
	}
	return s.Reindex(id)
}

func (s *Store) Reindex(id string) error {
	docs, err := s.Documents(id)
	if err != nil {
		return err
	}
	for _, d := range docs {
		s.stopIndexing(d.ID)
		_, _ = s.db.Exec(`UPDATE rag_documents SET status = 'queued', error = '' WHERE id = ?`, d.ID)
		s.queue <- d.ID
	}
	s.changed()
	return nil
}

// ---------------------------------------------------------------- documents

func (s *Store) Documents(collectionID string) ([]Document, error) {
	rows, err := s.db.Query(`SELECT id, collection_id, name, path, size, chars, chunks, status, error, created_at
FROM rag_documents WHERE collection_id = ? ORDER BY id`, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.CollectionID, &d.Name, &d.Path, &d.Size, &d.Chars, &d.Chunks, &d.Status, &d.Error, &d.CreatedAt); err != nil {
			return nil, err
		}
		if d.Status == "ready" {
			d.Progress = 1
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) document(id int64) (Document, error) {
	var d Document
	err := s.db.QueryRow(`SELECT id, collection_id, name, path, size, chars, chunks, status, error, created_at
FROM rag_documents WHERE id = ?`, id).Scan(&d.ID, &d.CollectionID, &d.Name, &d.Path, &d.Size, &d.Chars, &d.Chunks, &d.Status, &d.Error, &d.CreatedAt)
	return d, err
}

func (s *Store) queuedIDs() ([]int64, error) {
	rows, err := s.db.Query(`SELECT id FROM rag_documents WHERE status = 'queued' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// AddFiles queues files for indexing and returns the created documents.
// Unsupported files are skipped and reported in the error.
func (s *Store) AddFiles(collectionID string, files []string) ([]Document, error) {
	if _, err := s.Collection(collectionID); err != nil {
		return nil, errors.New(i18n.T("coleção não encontrada"))
	}
	var docs []Document
	var skipped []string
	for _, path := range files {
		st, err := os.Stat(path)
		if err != nil || st.IsDir() || !Supported(path) {
			skipped = append(skipped, filepath.Base(path))
			continue
		}
		res, err := s.db.Exec(`INSERT INTO rag_documents (collection_id, name, path, size, status, created_at) VALUES (?, ?, ?, ?, 'queued', ?)`,
			collectionID, filepath.Base(path), path, st.Size(), now())
		if err != nil {
			return docs, err
		}
		id, _ := res.LastInsertId()
		d, _ := s.document(id)
		docs = append(docs, d)
		s.queue <- id
	}
	_, _ = s.db.Exec(`UPDATE rag_collections SET updated_at = ? WHERE id = ?`, now(), collectionID)
	s.changed()
	if len(skipped) > 0 {
		return docs, errors.New(i18n.Tf("formato não suportado: %s", strings.Join(skipped, ", ")))
	}
	return docs, nil
}

func (s *Store) RemoveDocument(id int64) error {
	d, err := s.document(id)
	if err != nil {
		return err
	}
	s.stopIndexing(id)
	_, err = s.db.Exec(`DELETE FROM rag_documents WHERE id = ?`, id)
	s.invalidate(d.CollectionID)
	s.changed()
	return err
}

func (s *Store) stopIndexing(id int64) {
	s.mu.Lock()
	if cancel := s.cancel[id]; cancel != nil {
		cancel()
	}
	s.mu.Unlock()
}

func (s *Store) setStatus(d *Document, status, errMsg string, progress float64) {
	d.Status, d.Error, d.Progress = status, errMsg, progress
	_, _ = s.db.Exec(`UPDATE rag_documents SET status = ?, error = ?, chars = ?, chunks = ? WHERE id = ?`,
		status, errMsg, d.Chars, d.Chunks, d.ID)
	s.emitDoc(*d)
}

func (s *Store) worker() {
	for id := range s.queue {
		d, err := s.document(id)
		if err != nil || d.Status != "queued" {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		s.mu.Lock()
		s.cancel[id] = cancel
		s.mu.Unlock()
		s.index(ctx, &d)
		s.mu.Lock()
		delete(s.cancel, id)
		s.mu.Unlock()
		cancel()
		s.invalidate(d.CollectionID)
		s.changed()
	}
}

func (s *Store) index(ctx context.Context, d *Document) {
	c, err := s.Collection(d.CollectionID)
	if err != nil {
		return
	}
	s.setStatus(d, "indexing", "", 0)
	text, err := ExtractText(d.Path)
	if err != nil {
		s.setStatus(d, "error", err.Error(), 0)
		return
	}
	chunks := Chunk(text, chunkSize, chunkOverlap)
	if len(chunks) == 0 {
		s.setStatus(d, "error", i18n.T("nenhum texto encontrado no arquivo"), 0)
		return
	}
	d.Chars = len([]rune(text))
	docPrefix, _ := s.embedder.Prefixes(c.EmbedModel)

	var rows [][]byte
	for i := 0; i < len(chunks); i += batchSize {
		batch := chunks[i:min(i+batchSize, len(chunks))]
		inputs := make([]string, len(batch))
		for j, t := range batch {
			inputs[j] = docPrefix + d.Name + "\n" + t
		}
		vecs, err := s.embedder.Embed(ctx, c.EmbedModel, inputs)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.setStatus(d, "error", err.Error(), 0)
			return
		}
		for _, v := range vecs {
			rows = append(rows, encode(normalizeVec(v)))
		}
		s.setStatus(d, "indexing", "", float64(min(i+batchSize, len(chunks)))/float64(len(chunks)))
	}

	tx, err := s.db.Begin()
	if err != nil {
		s.setStatus(d, "error", err.Error(), 0)
		return
	}
	_, _ = tx.Exec(`DELETE FROM rag_chunks WHERE document_id = ?`, d.ID)
	for i, t := range chunks {
		if _, err := tx.Exec(`INSERT INTO rag_chunks (document_id, collection_id, idx, text, embedding) VALUES (?, ?, ?, ?, ?)`,
			d.ID, d.CollectionID, i, t, rows[i]); err != nil {
			_ = tx.Rollback()
			s.setStatus(d, "error", err.Error(), 0)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		s.setStatus(d, "error", err.Error(), 0)
		return
	}
	d.Chunks = len(chunks)
	s.setStatus(d, "ready", "", 1)
}

// ---------------------------------------------------------------- search

func (s *Store) invalidate(collectionID string) {
	s.mu.Lock()
	delete(s.cache, collectionID)
	s.mu.Unlock()
}

func (s *Store) vectors(collectionID string) ([]vector, error) {
	s.mu.Lock()
	cached, ok := s.cache[collectionID]
	s.mu.Unlock()
	if ok {
		return cached, nil
	}
	rows, err := s.db.Query(`SELECT id, document_id, embedding FROM rag_chunks WHERE collection_id = ?`, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []vector
	for rows.Next() {
		var v vector
		var blob []byte
		if err := rows.Scan(&v.chunkID, &v.docID, &blob); err != nil {
			return nil, err
		}
		v.vec = decode(blob)
		out = append(out, v)
	}
	s.mu.Lock()
	s.cache[collectionID] = out
	s.mu.Unlock()
	return out, rows.Err()
}

// Search returns the k passages most similar to query.
func (s *Store) Search(ctx context.Context, collectionID, query string, k int) ([]Hit, error) {
	c, err := s.Collection(collectionID)
	if err != nil {
		return nil, errors.New(i18n.T("coleção não encontrada"))
	}
	vecs, err := s.vectors(collectionID)
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return []Hit{}, nil
	}
	_, queryPrefix := s.embedder.Prefixes(c.EmbedModel)
	q, err := s.embedder.Embed(ctx, c.EmbedModel, []string{queryPrefix + query})
	if err != nil {
		return nil, err
	}
	qv := normalizeVec(q[0])

	type scored struct {
		v     vector
		score float64
	}
	all := make([]scored, 0, len(vecs))
	for _, v := range vecs {
		if len(v.vec) != len(qv) {
			continue // indexed with another model; needs re-index
		}
		var dot float64
		for i := range qv {
			dot += float64(qv[i]) * float64(v.vec[i])
		}
		all = append(all, scored{v, dot})
	}
	if len(all) == 0 && len(vecs) > 0 {
		return nil, errors.New(i18n.T("a coleção foi indexada com outro modelo — reindexe-a"))
	}
	sort.Slice(all, func(i, j int) bool { return all[i].score > all[j].score })
	if len(all) > k {
		all = all[:k]
	}
	hits := make([]Hit, 0, len(all))
	for _, a := range all {
		var h Hit
		err := s.db.QueryRow(`SELECT k.document_id, d.name, k.idx, k.text FROM rag_chunks k JOIN rag_documents d ON d.id = k.document_id WHERE k.id = ?`,
			a.v.chunkID).Scan(&h.DocumentID, &h.Document, &h.Chunk, &h.Text)
		if err != nil {
			continue
		}
		h.Score = a.score
		hits = append(hits, h)
	}
	return hits, nil
}

// ---------------------------------------------------------------- vectors

func normalizeVec(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	n := math.Sqrt(sum)
	if n == 0 {
		return v
	}
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) / n)
	}
	return out
}

func encode(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(x))
	}
	return b
}

func decode(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v
}
