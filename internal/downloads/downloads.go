// Package downloads fetches large files with resume support. Each download
// is identified by a key and may contain several files; files are written to
// <dest>.part and renamed when complete.
package downloads

import (
	"context"
	"errors"
	"fmt"
	"github.com/devalexandre/llamafile-ui/internal/i18n"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type File struct {
	URL  string
	Dest string
	Size int64 // expected size, 0 if unknown
}

type Status struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Received int64   `json:"received"`
	Total    int64   `json:"total"`
	Progress float64 `json:"progress"`
	Speed    float64 `json:"speed"` // bytes/s
	ETA      float64 `json:"eta"`   // seconds, -1 if unknown
	File     int     `json:"file"`
	Files    int     `json:"files"`
}

type task struct {
	key, label string
	files      []File
	total      int64
	done       int64 // bytes of finished files
	current    int64 // bytes of the current file on disk
	index      int
	speed      float64
	cancel     context.CancelFunc
}

type Manager struct {
	mu       sync.Mutex
	tasks    map[string]*task
	client   *http.Client
	OnChange func([]Status)              // called (throttled) while downloads progress
	OnDone   func(key string, err error) // called once per Start
}

var ErrCanceled = errors.New("download cancelado")

func New() *Manager {
	return &Manager{
		tasks:  map[string]*task{},
		client: &http.Client{Timeout: 0},
	}
}

func (m *Manager) Active() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Status, 0, len(m.tasks))
	for _, t := range m.tasks {
		out = append(out, t.status())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func (m *Manager) IsActive(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.tasks[key]
	return ok
}

func (t *task) status() Status {
	got := t.done + t.current
	s := Status{Key: t.key, Label: t.label, Received: got, Total: t.total, Speed: t.speed, ETA: -1,
		File: t.index + 1, Files: len(t.files)}
	if t.total > 0 {
		s.Progress = float64(got) / float64(t.total)
	}
	if t.speed > 0 && t.total > got {
		s.ETA = float64(t.total-got) / t.speed
	}
	return s
}

// Start begins downloading files under key. Files already complete are skipped.
func (m *Manager) Start(key, label string, files []File) error {
	m.mu.Lock()
	if _, ok := m.tasks[key]; ok {
		m.mu.Unlock()
		return nil
	}
	t := &task{key: key, label: label}
	for _, f := range files {
		if _, err := os.Stat(f.Dest); err == nil {
			continue
		}
		t.files = append(t.files, f)
		t.total += f.Size
	}
	if len(t.files) == 0 {
		m.mu.Unlock()
		m.done(key, nil)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	m.tasks[key] = t
	m.mu.Unlock()

	m.changed()
	go func() {
		err := m.run(ctx, t)
		if ctx.Err() != nil {
			err = ErrCanceled
		}
		m.mu.Lock()
		delete(m.tasks, key)
		m.mu.Unlock()
		m.changed()
		m.done(key, err)
	}()
	return nil
}

// Cancel stops a download. Partial files are kept so it can resume later.
func (m *Manager) Cancel(key string) {
	m.mu.Lock()
	t, ok := m.tasks[key]
	m.mu.Unlock()
	if ok {
		t.cancel()
	}
}

func (m *Manager) changed() {
	if m.OnChange != nil {
		m.OnChange(m.Active())
	}
}

func (m *Manager) done(key string, err error) {
	if m.OnDone != nil {
		m.OnDone(key, err)
	}
}

func (m *Manager) run(ctx context.Context, t *task) error {
	for i, f := range t.files {
		m.mu.Lock()
		t.index = i
		m.mu.Unlock()
		if err := m.fetch(ctx, t, f); err != nil {
			return err
		}
		m.mu.Lock()
		t.done += f.Size
		t.current = 0
		m.mu.Unlock()
	}
	return nil
}

func (m *Manager) fetch(ctx context.Context, t *task, f File) error {
	if err := os.MkdirAll(filepath.Dir(f.Dest), 0o755); err != nil {
		return err
	}
	part := f.Dest + ".part"
	out, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf(i18n.T("não foi possível gravar %s: %w"), part, err)
	}
	defer out.Close()
	st, _ := out.Stat()
	existing := st.Size()
	if f.Size > 0 && existing > f.Size {
		_ = out.Truncate(0)
		existing = 0
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "LlamafileUI/0.1")
	if existing > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existing))
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if existing > 0 { // server ignored Range: start over
			if err := out.Truncate(0); err != nil {
				return err
			}
			existing = 0
		}
	case http.StatusPartialContent:
	case http.StatusRequestedRangeNotSatisfiable:
		if f.Size > 0 && existing == f.Size {
			break // already complete
		}
		return errors.New(i18n.T("servidor recusou retomar o download (HTTP 416)"))
	default:
		return errors.New(i18n.Tf("HTTP %d ao baixar %s", resp.StatusCode, filepath.Base(f.Dest)))
	}

	m.mu.Lock()
	t.current = existing
	m.mu.Unlock()

	buf := make([]byte, 1<<20)
	lastTick := time.Now()
	var lastBytes = existing
	written := existing
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			m.mu.Lock()
			t.current = written
			if dt := time.Since(lastTick); dt >= 500*time.Millisecond {
				inst := float64(written-lastBytes) / dt.Seconds()
				if t.speed == 0 {
					t.speed = inst
				} else {
					t.speed = t.speed*0.7 + inst*0.3
				}
				lastBytes, lastTick = written, time.Now()
				m.mu.Unlock()
				m.changed()
			} else {
				m.mu.Unlock()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	if f.Size > 0 && written != f.Size {
		return errors.New(i18n.Tf("download incompleto de %s (%d de %d bytes)", filepath.Base(f.Dest), written, f.Size))
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(part, f.Dest)
}
