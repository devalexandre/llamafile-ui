// Package eventbus delivers backend events to the frontend over HTTP
// long-polling (GET /_bus/events?after=N).
//
// Wails v3 (beta) delivers events by injecting JavaScript into the webview.
// With WebKitGTK 6 that path was observed to stop running injected scripts
// once the app spawned a child process (the llamafile server), while HTTP
// requests kept working. Polling over the asset server avoids that and behaves
// the same on every platform.
package eventbus

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Path is the endpoint served by [Bus.Handler].
const Path = "/_bus/events"

const (
	capacity    = 4096             // events kept for clients that fall behind
	pollTimeout = 25 * time.Second // how long a request waits for new events
)

type Event struct {
	Seq  uint64          `json:"seq"`
	Name string          `json:"name"`
	Data json.RawMessage `json:"data,omitempty"`
}

type Bus struct {
	mu     sync.Mutex
	seq    uint64
	events []Event       // ring of the most recent events, ordered by Seq
	notify chan struct{} // closed and replaced on every publish
}

func New() *Bus {
	return &Bus{notify: make(chan struct{})}
}

// Publish queues an event for every connected frontend.
func (b *Bus) Publish(name string, data any) {
	var raw json.RawMessage
	if data != nil {
		var err error
		if raw, err = json.Marshal(data); err != nil {
			return
		}
	}
	b.mu.Lock()
	b.seq++
	b.events = append(b.events, Event{Seq: b.seq, Name: name, Data: raw})
	if over := len(b.events) - capacity; over > 0 {
		b.events = append(b.events[:0:0], b.events[over:]...)
	}
	close(b.notify)
	b.notify = make(chan struct{})
	b.mu.Unlock()
}

// since returns the events after seq, the latest seq and a channel that is
// closed when something new is published.
func (b *Bus) since(after uint64) ([]Event, uint64, <-chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []Event
	for i := len(b.events) - 1; i >= 0 && b.events[i].Seq > after; i-- {
		out = append(out, b.events[i])
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, b.seq, b.notify
}

// Wait blocks until there are events after seq (or ctx/timeout ends).
// after < 0 means "start from now": it returns the current seq immediately.
func (b *Bus) Wait(ctx context.Context, after int64) ([]Event, uint64) {
	if after < 0 {
		_, latest, _ := b.since(0)
		return []Event{}, latest
	}
	timer := time.NewTimer(pollTimeout)
	defer timer.Stop()
	for {
		events, latest, notify := b.since(uint64(after))
		if len(events) > 0 || uint64(after) > latest { // app restarted: resync
			if events == nil {
				events = []Event{}
			}
			return events, latest
		}
		select {
		case <-notify:
		case <-timer.C:
			return []Event{}, latest
		case <-ctx.Done():
			return []Event{}, latest
		}
	}
}

// Handler serves the long-poll endpoint.
func (b *Bus) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		after, err := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		if err != nil {
			after = -1
		}
		events, latest := b.Wait(r.Context(), after)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(struct {
			Seq    uint64  `json:"seq"`
			Events []Event `json:"events"`
		}{latest, events})
	})
}
