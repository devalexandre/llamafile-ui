// Package chat persists conversations in SQLite and streams completions from
// the llamafile OpenAI-compatible server.
package chat

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const DefaultTitle = "Nova conversa"

type Conversation struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	ModelID      string `json:"modelId"`
	SystemPrompt string `json:"systemPrompt"`
	Pinned       bool   `json:"pinned"`
	CollectionID string `json:"collectionId"` // documents used for RAG ("" = none)
	AgentID      string `json:"agentId"`      // persona answering ("" = default assistant)
	ProjectID    string `json:"projectId"`    // project the conversation belongs to
	CreatedAt    int64  `json:"createdAt"`    // unix ms
	UpdatedAt    int64  `json:"updatedAt"`
	Preview      string `json:"preview"`
	MessageCount int    `json:"messageCount"`
}

type Message struct {
	ID             int64    `json:"id"`
	ConversationID string   `json:"conversationId"`
	Role           string   `json:"role"` // system, user, assistant
	Content        string   `json:"content"`
	Reasoning      string   `json:"reasoning"`
	ModelID        string   `json:"modelId"`
	Attachments    []string `json:"attachments"`
	TPS            float64  `json:"tps"`
	Tokens         int      `json:"tokens"`
	Error          string   `json:"error"`
	Sources        []Source `json:"sources"` // passages retrieved from documents
	CreatedAt      int64    `json:"createdAt"`
}

// Source is a document passage used to answer (RAG).
type Source struct {
	N        int     `json:"n"`
	Document string  `json:"document"`
	Chunk    int     `json:"chunk"`
	Text     string  `json:"text"`
	Score    float64 `json:"score"`
}

type Store struct{ db *sql.DB }

const schema = `
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS conversations (
    id            TEXT PRIMARY KEY,
    title         TEXT NOT NULL,
    model_id      TEXT NOT NULL DEFAULT '',
    system_prompt TEXT NOT NULL DEFAULT '',
    pinned        INTEGER NOT NULL DEFAULT 0,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL,
    content         TEXT NOT NULL DEFAULT '',
    reasoning       TEXT NOT NULL DEFAULT '',
    model_id        TEXT NOT NULL DEFAULT '',
    attachments     TEXT NOT NULL DEFAULT '[]',
    tps             REAL NOT NULL DEFAULT 0,
    tokens          INTEGER NOT NULL DEFAULT 0,
    error           TEXT NOT NULL DEFAULT '',
    created_at      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, id);
CREATE INDEX IF NOT EXISTS idx_conversations_updated ON conversations(updated_at DESC);
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema + agentsSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("chat db: %w", err)
	}
	// Columns added after the first release.
	for _, m := range []string{
		`ALTER TABLE conversations ADD COLUMN collection_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN sources TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE conversations ADD COLUMN agent_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE conversations ADD COLUMN project_id TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := db.Exec(m); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			db.Close()
			return nil, fmt.Errorf("chat db migration: %w", err)
		}
	}
	return &Store{db: db}, nil
}

// DB exposes the connection so other features share the same database file.
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Close() error { return s.db.Close() }

func now() int64 { return time.Now().UnixMilli() }

// List returns conversations, pinned first then most recent. search matches
// titles and message contents. Conversations inside projects only show up
// when searching; use ProjectConversations for them.
func (s *Store) List(search string) ([]Conversation, error) {
	return s.list(search, nil)
}

// ProjectConversations lists the conversations of a project.
func (s *Store) ProjectConversations(projectID string) ([]Conversation, error) {
	return s.list("", &projectID)
}

func (s *Store) list(search string, projectID *string) ([]Conversation, error) {
	query := `
SELECT c.id, c.title, c.model_id, c.system_prompt, c.pinned, c.collection_id, c.agent_id, c.project_id, c.created_at, c.updated_at,
       COALESCE((SELECT substr(m.content, 1, 120) FROM messages m
                 WHERE m.conversation_id = c.id AND m.role = 'user' ORDER BY m.id DESC LIMIT 1), ''),
       (SELECT COUNT(*) FROM messages m WHERE m.conversation_id = c.id)
FROM conversations c`
	var args []any
	switch q := strings.TrimSpace(search); {
	case projectID != nil:
		query += ` WHERE c.project_id = ?`
		args = append(args, *projectID)
	case q != "":
		like := "%" + q + "%"
		query += ` WHERE c.title LIKE ? OR EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id = c.id AND m.content LIKE ?)`
		args = append(args, like, like)
	default:
		query += ` WHERE c.project_id = ''`
	}
	query += ` ORDER BY c.pinned DESC, c.updated_at DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Conversation{}
	for rows.Next() {
		var c Conversation
		var pinned int
		if err := rows.Scan(&c.ID, &c.Title, &c.ModelID, &c.SystemPrompt, &pinned, &c.CollectionID, &c.AgentID, &c.ProjectID,
			&c.CreatedAt, &c.UpdatedAt, &c.Preview, &c.MessageCount); err != nil {
			return nil, err
		}
		c.Pinned = pinned != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) Get(id string) (Conversation, error) {
	var c Conversation
	var pinned int
	err := s.db.QueryRow(`SELECT id, title, model_id, system_prompt, pinned, collection_id, agent_id, project_id, created_at, updated_at
FROM conversations WHERE id = ?`, id).Scan(&c.ID, &c.Title, &c.ModelID, &c.SystemPrompt, &pinned, &c.CollectionID, &c.AgentID,
		&c.ProjectID, &c.CreatedAt, &c.UpdatedAt)
	c.Pinned = pinned != 0
	return c, err
}

func (s *Store) Messages(conversationID string) ([]Message, error) {
	rows, err := s.db.Query(`SELECT id, conversation_id, role, content, reasoning, model_id, attachments, tps, tokens, error, sources, created_at
FROM messages WHERE conversation_id = ? ORDER BY id`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var att, src string
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.Reasoning, &m.ModelID, &att,
			&m.TPS, &m.Tokens, &m.Error, &src, &m.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(att), &m.Attachments)
		if m.Attachments == nil {
			m.Attachments = []string{}
		}
		_ = json.Unmarshal([]byte(src), &m.Sources)
		if m.Sources == nil {
			m.Sources = []Source{}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) Create(modelID, systemPrompt string) (Conversation, error) {
	return s.CreateWith(Conversation{ModelID: modelID, SystemPrompt: systemPrompt})
}

// CreateWith starts a conversation from a template (model, agent, project…).
// Inside a project the project's agent and documents apply by default.
func (s *Store) CreateWith(c Conversation) (Conversation, error) {
	if c.ProjectID != "" {
		if p, err := s.GetProject(c.ProjectID); err == nil {
			if c.AgentID == "" {
				c.AgentID = p.AgentID
			}
			if c.CollectionID == "" {
				c.CollectionID = p.CollectionID
			}
		}
	}
	if a, ok := s.agentFor(c); ok && c.CollectionID == "" {
		c.CollectionID = a.CollectionID
	}
	t := now()
	c.ID, c.Title, c.CreatedAt, c.UpdatedAt = uuid.NewString(), DefaultTitle, t, t
	_, err := s.db.Exec(`INSERT INTO conversations (id, title, model_id, system_prompt, pinned, collection_id, agent_id, project_id, created_at, updated_at)
VALUES (?, ?, ?, ?, 0, ?, ?, ?, ?, ?)`, c.ID, c.Title, c.ModelID, c.SystemPrompt, c.CollectionID, c.AgentID, c.ProjectID, c.CreatedAt, c.UpdatedAt)
	return c, err
}

func (s *Store) AddMessage(m Message) (Message, error) {
	if m.Attachments == nil {
		m.Attachments = []string{}
	}
	if m.Sources == nil {
		m.Sources = []Source{}
	}
	att, _ := json.Marshal(m.Attachments)
	src, _ := json.Marshal(m.Sources)
	m.CreatedAt = now()
	res, err := s.db.Exec(`INSERT INTO messages (conversation_id, role, content, reasoning, model_id, attachments, tps, tokens, error, sources, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, m.ConversationID, m.Role, m.Content, m.Reasoning, m.ModelID, string(att),
		m.TPS, m.Tokens, m.Error, string(src), m.CreatedAt)
	if err != nil {
		return m, err
	}
	m.ID, _ = res.LastInsertId()
	_, err = s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, m.CreatedAt, m.ConversationID)
	return m, err
}

func (s *Store) UpdateMessage(m Message) error {
	if m.Sources == nil {
		m.Sources = []Source{}
	}
	if m.Attachments == nil {
		m.Attachments = []string{}
	}
	src, _ := json.Marshal(m.Sources)
	att, _ := json.Marshal(m.Attachments)
	_, err := s.db.Exec(`UPDATE messages SET content = ?, reasoning = ?, tps = ?, tokens = ?, error = ?, sources = ?, attachments = ? WHERE id = ?`,
		m.Content, m.Reasoning, m.TPS, m.Tokens, m.Error, string(src), string(att), m.ID)
	return err
}

// DeleteFrom removes a message and everything after it (edit / regenerate).
func (s *Store) DeleteFrom(conversationID string, messageID int64) error {
	_, err := s.db.Exec(`DELETE FROM messages WHERE conversation_id = ? AND id >= ?`, conversationID, messageID)
	return err
}

func (s *Store) Rename(id, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		title = DefaultTitle
	}
	_, err := s.db.Exec(`UPDATE conversations SET title = ? WHERE id = ?`, title, id)
	return err
}

func (s *Store) SetPinned(id string, pinned bool) error {
	v := 0
	if pinned {
		v = 1
	}
	_, err := s.db.Exec(`UPDATE conversations SET pinned = ? WHERE id = ?`, v, id)
	return err
}

func (s *Store) SetModel(id, modelID string) error {
	_, err := s.db.Exec(`UPDATE conversations SET model_id = ? WHERE id = ?`, modelID, id)
	return err
}

func (s *Store) SetCollection(id, collectionID string) error {
	_, err := s.db.Exec(`UPDATE conversations SET collection_id = ? WHERE id = ?`, collectionID, id)
	return err
}

func (s *Store) SetSystemPrompt(id, prompt string) error {
	_, err := s.db.Exec(`UPDATE conversations SET system_prompt = ? WHERE id = ?`, prompt, id)
	return err
}

func (s *Store) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM conversations WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteAll() error {
	_, err := s.db.Exec(`DELETE FROM conversations`)
	return err
}

// Export renders a conversation as Markdown or JSON.
func (s *Store) Export(id, format string) (string, error) {
	c, err := s.Get(id)
	if err != nil {
		return "", err
	}
	msgs, err := s.Messages(id)
	if err != nil {
		return "", err
	}
	if format == "json" {
		data, err := json.MarshalIndent(map[string]any{"conversation": c, "messages": msgs}, "", "  ")
		return string(data), err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", c.Title)
	fmt.Fprintf(&b, "_Modelo: %s · %s_\n\n", c.ModelID, time.UnixMilli(c.CreatedAt).Format("02/01/2006 15:04"))
	for _, m := range msgs {
		who := "Você"
		if m.Role == "assistant" {
			who = "Assistente"
		}
		fmt.Fprintf(&b, "---\n\n**%s**\n\n%s\n\n", who, m.Content)
		for _, src := range m.Sources {
			fmt.Fprintf(&b, "> [%d] %s (trecho %d)\n", src.N, src.Document, src.Chunk+1)
		}
	}
	return b.String(), nil
}
