package eventbus

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWaitReturnsPublishedEvents(t *testing.T) {
	b := New()
	_, start := b.Wait(context.Background(), -1)

	go func() {
		time.Sleep(50 * time.Millisecond)
		b.Publish("chat:delta", map[string]string{"content": "olá"})
		b.Publish("chat:done", nil)
	}()

	events, latest := b.Wait(context.Background(), int64(start))
	if len(events) == 0 || events[0].Name != "chat:delta" {
		t.Fatalf("expected chat:delta first, got %+v", events)
	}
	if string(events[0].Data) != `{"content":"olá"}` {
		t.Fatalf("unexpected data %s", events[0].Data)
	}
	// Anything not yet seen is returned on the next poll, in order.
	all := events
	if len(all) < 2 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		rest, _ := b.Wait(ctx, int64(events[len(events)-1].Seq))
		all = append(all, rest...)
	}
	if all[len(all)-1].Name != "chat:done" || latest < events[0].Seq {
		t.Fatalf("events out of order: %+v", all)
	}
}

func TestWaitHonoursContext(t *testing.T) {
	b := New()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	events, _ := b.Wait(ctx, 0)
	if len(events) != 0 || time.Since(start) > time.Second {
		t.Fatalf("expected an empty, prompt return; got %d events after %v", len(events), time.Since(start))
	}
}

func TestRingCapacityAndHandler(t *testing.T) {
	b := New()
	for i := 0; i < capacity+10; i++ {
		b.Publish("tick", i)
	}
	rec := httptest.NewRecorder()
	b.Handler().ServeHTTP(rec, httptest.NewRequest("GET", Path+"?after=0", nil))
	var resp struct {
		Seq    uint64  `json:"seq"`
		Events []Event `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Events) != capacity || resp.Seq != capacity+10 {
		t.Fatalf("got %d events, seq %d", len(resp.Events), resp.Seq)
	}
	if resp.Events[0].Seq != 11 {
		t.Fatalf("oldest kept event should be seq 11, got %d", resp.Events[0].Seq)
	}
}
