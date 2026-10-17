package chat

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// Agent is a reusable assistant persona: its instructions replace the default
// system prompt, and it may pin a model, a temperature and a document collection.
type Agent struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Icon         string  `json:"icon"` // emoji
	Description  string  `json:"description"`
	SystemPrompt string  `json:"systemPrompt"`
	ModelID      string  `json:"modelId"`      // "" = the model chosen in the chat
	Temperature  float64 `json:"temperature"`  // < 0 = app default
	CollectionID string  `json:"collectionId"` // documents always used by the agent
	CreatedAt    int64   `json:"createdAt"`
	UpdatedAt    int64   `json:"updatedAt"`
	Uses         int     `json:"uses"` // conversations using it
}

// Project groups conversations that share an agent and documents.
type Project struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Icon          string `json:"icon"`
	Description   string `json:"description"`
	AgentID       string `json:"agentId"`
	CollectionID  string `json:"collectionId"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
	Conversations int    `json:"conversations"`
}

const agentsSchema = `
CREATE TABLE IF NOT EXISTS agents (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    icon          TEXT NOT NULL DEFAULT '',
    description   TEXT NOT NULL DEFAULT '',
    system_prompt TEXT NOT NULL DEFAULT '',
    model_id      TEXT NOT NULL DEFAULT '',
    temperature   REAL NOT NULL DEFAULT -1,
    collection_id TEXT NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS projects (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    icon          TEXT NOT NULL DEFAULT '',
    description   TEXT NOT NULL DEFAULT '',
    agent_id      TEXT NOT NULL DEFAULT '',
    collection_id TEXT NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);`

// ------------------------------------------------------------------ agents

func (s *Store) ListAgents() ([]Agent, error) {
	rows, err := s.db.Query(`SELECT a.id, a.name, a.icon, a.description, a.system_prompt, a.model_id, a.temperature,
       a.collection_id, a.created_at, a.updated_at,
       (SELECT COUNT(*) FROM conversations c WHERE c.agent_id = a.id)
FROM agents a ORDER BY lower(a.name)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Agent{}
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.Icon, &a.Description, &a.SystemPrompt, &a.ModelID, &a.Temperature,
			&a.CollectionID, &a.CreatedAt, &a.UpdatedAt, &a.Uses); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetAgent(id string) (Agent, error) {
	var a Agent
	err := s.db.QueryRow(`SELECT id, name, icon, description, system_prompt, model_id, temperature, collection_id, created_at, updated_at
FROM agents WHERE id = ?`, id).Scan(&a.ID, &a.Name, &a.Icon, &a.Description, &a.SystemPrompt, &a.ModelID, &a.Temperature,
		&a.CollectionID, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

// SaveAgent creates (empty ID) or updates an agent.
func (s *Store) SaveAgent(a Agent) (Agent, error) {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		return a, errors.New("dê um nome ao agente")
	}
	if strings.TrimSpace(a.SystemPrompt) == "" {
		return a, errors.New("escreva as instruções do agente")
	}
	if a.Temperature > 2 {
		a.Temperature = 2
	}
	t := now()
	a.UpdatedAt = t
	if a.ID == "" {
		a.ID, a.CreatedAt = uuid.NewString(), t
		_, err := s.db.Exec(`INSERT INTO agents (id, name, icon, description, system_prompt, model_id, temperature, collection_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, a.ID, a.Name, a.Icon, a.Description, a.SystemPrompt, a.ModelID, a.Temperature,
			a.CollectionID, a.CreatedAt, a.UpdatedAt)
		return a, err
	}
	_, err := s.db.Exec(`UPDATE agents SET name = ?, icon = ?, description = ?, system_prompt = ?, model_id = ?, temperature = ?,
collection_id = ?, updated_at = ? WHERE id = ?`, a.Name, a.Icon, a.Description, a.SystemPrompt, a.ModelID, a.Temperature,
		a.CollectionID, a.UpdatedAt, a.ID)
	return a, err
}

// DeleteAgent removes an agent; its conversations and projects keep working
// with the default assistant.
func (s *Store) DeleteAgent(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, q := range []string{
		`UPDATE conversations SET agent_id = '' WHERE agent_id = ?`,
		`UPDATE projects SET agent_id = '' WHERE agent_id = ?`,
		`DELETE FROM agents WHERE id = ?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---------------------------------------------------------------- projects

func (s *Store) ListProjects() ([]Project, error) {
	rows, err := s.db.Query(`SELECT p.id, p.name, p.icon, p.description, p.agent_id, p.collection_id, p.created_at,
       MAX(p.updated_at, COALESCE((SELECT MAX(c.updated_at) FROM conversations c WHERE c.project_id = p.id), 0)),
       (SELECT COUNT(*) FROM conversations c WHERE c.project_id = p.id)
FROM projects p ORDER BY 8 DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Icon, &p.Description, &p.AgentID, &p.CollectionID, &p.CreatedAt,
			&p.UpdatedAt, &p.Conversations); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetProject(id string) (Project, error) {
	var p Project
	err := s.db.QueryRow(`SELECT id, name, icon, description, agent_id, collection_id, created_at, updated_at
FROM projects WHERE id = ?`, id).Scan(&p.ID, &p.Name, &p.Icon, &p.Description, &p.AgentID, &p.CollectionID, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// SaveProject creates (empty ID) or updates a project. Changing its agent
// also updates the conversations inside it.
func (s *Store) SaveProject(p Project) (Project, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return p, errors.New("dê um nome ao projeto")
	}
	t := now()
	p.UpdatedAt = t
	if p.ID == "" {
		p.ID, p.CreatedAt = uuid.NewString(), t
		_, err := s.db.Exec(`INSERT INTO projects (id, name, icon, description, agent_id, collection_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, p.ID, p.Name, p.Icon, p.Description, p.AgentID, p.CollectionID, p.CreatedAt, p.UpdatedAt)
		return p, err
	}
	if _, err := s.db.Exec(`UPDATE projects SET name = ?, icon = ?, description = ?, agent_id = ?, collection_id = ?, updated_at = ?
WHERE id = ?`, p.Name, p.Icon, p.Description, p.AgentID, p.CollectionID, p.UpdatedAt, p.ID); err != nil {
		return p, err
	}
	_, err := s.db.Exec(`UPDATE conversations SET agent_id = ? WHERE project_id = ?`, p.AgentID, p.ID)
	return p, err
}

// DeleteProject removes a project. Its conversations are deleted too when
// withConversations is set; otherwise they move to the main list.
func (s *Store) DeleteProject(id string, withConversations bool) error {
	q := `UPDATE conversations SET project_id = '' WHERE project_id = ?`
	if withConversations {
		q = `DELETE FROM conversations WHERE project_id = ?`
	}
	if _, err := s.db.Exec(q, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM projects WHERE id = ?`, id)
	return err
}

// SetAgent changes the agent of a conversation.
func (s *Store) SetAgent(conversationID, agentID string) error {
	_, err := s.db.Exec(`UPDATE conversations SET agent_id = ? WHERE id = ?`, agentID, conversationID)
	return err
}

// SetProject moves a conversation into a project ("" = out of any project).
func (s *Store) SetProject(conversationID, projectID string) error {
	agent := ""
	if projectID != "" {
		p, err := s.GetProject(projectID)
		if err != nil {
			return err
		}
		agent = p.AgentID
	}
	_, err := s.db.Exec(`UPDATE conversations SET project_id = ?, agent_id = CASE WHEN ? != '' THEN ? ELSE agent_id END WHERE id = ?`,
		projectID, agent, agent, conversationID)
	return err
}

// agentFor returns the agent of a conversation, if any.
func (s *Store) agentFor(c Conversation) (Agent, bool) {
	if c.AgentID == "" {
		return Agent{}, false
	}
	a, err := s.GetAgent(c.AgentID)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		return Agent{}, false
	}
	return a, true
}
