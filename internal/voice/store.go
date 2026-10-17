package voice

import (
	"database/sql"
	"fmt"
	"time"
)

// Transcript is a saved transcription (from a recording or an audio file).
type Transcript struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source"` // recording, file
	ModelID    string `json:"modelId"`
	Language   string `json:"language"`
	DurationMs int64  `json:"durationMs"` // processing time
	Text       string `json:"text"`
	SRT        string `json:"srt"`
	CreatedAt  int64  `json:"createdAt"`
}

type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS transcripts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    source      TEXT NOT NULL,
    model_id    TEXT NOT NULL,
    language    TEXT NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    text        TEXT NOT NULL,
    srt         TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL
);`

func OpenStore(db *sql.DB) (*Store, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("transcripts db: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Add(t Transcript) (Transcript, error) {
	t.CreatedAt = time.Now().UnixMilli()
	res, err := s.db.Exec(`INSERT INTO transcripts (name, source, model_id, language, duration_ms, text, srt, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, t.Name, t.Source, t.ModelID, t.Language, t.DurationMs, t.Text, t.SRT, t.CreatedAt)
	if err != nil {
		return t, err
	}
	t.ID, _ = res.LastInsertId()
	return t, nil
}

func (s *Store) List() ([]Transcript, error) {
	rows, err := s.db.Query(`SELECT id, name, source, model_id, language, duration_ms, text, srt, created_at FROM transcripts ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Transcript{}
	for rows.Next() {
		var t Transcript
		if err := rows.Scan(&t.ID, &t.Name, &t.Source, &t.ModelID, &t.Language, &t.DurationMs, &t.Text, &t.SRT, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) Get(id int64) (Transcript, error) {
	var t Transcript
	err := s.db.QueryRow(`SELECT id, name, source, model_id, language, duration_ms, text, srt, created_at FROM transcripts WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.Source, &t.ModelID, &t.Language, &t.DurationMs, &t.Text, &t.SRT, &t.CreatedAt)
	return t, err
}

func (s *Store) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM transcripts WHERE id = ?`, id)
	return err
}
