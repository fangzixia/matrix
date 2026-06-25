package stream

import (
	"context"
	"sync"
	"testing"
	"time"
)

type collectSink struct {
	mu   sync.Mutex
	msgs []Message
}

func (s *collectSink) Publish(_ context.Context, msg Message) error {
	s.mu.Lock()
	s.msgs = append(s.msgs, msg)
	s.mu.Unlock()
	return nil
}

func (s *collectSink) snapshot() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Message, len(s.msgs))
	copy(out, s.msgs)
	return out
}

func TestOutputCoalesceSink_MergesDeltas(t *testing.T) {
	inner := &collectSink{}
	c := NewOutputCoalesceSink(inner, "sess-1", 30*time.Millisecond)
	defer c.Close()

	ctx := context.Background()
	for _, part := range []string{"line1\n", "line2\n", "line3"} {
		if err := c.Publish(ctx, ToolOutputDelta("sess-1", "tu-1", "bash", part, 0)); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(80 * time.Millisecond)

	msgs := inner.snapshot()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 coalesced message, got %d", len(msgs))
	}
	if msgs[0].Data == nil || msgs[0].Data.Delta != "line1\nline2\nline3" {
		t.Fatalf("unexpected delta: %#v", msgs[0].Data)
	}
}

func TestOutputCoalesceSink_FlushesOnCompleted(t *testing.T) {
	inner := &collectSink{}
	c := NewOutputCoalesceSink(inner, "sess-1", time.Hour)
	defer c.Close()

	ctx := context.Background()
	if err := c.Publish(ctx, ToolOutputDelta("sess-1", "tu-2", "grep", "partial", 0)); err != nil {
		t.Fatal(err)
	}
	if err := c.Publish(ctx, ToolFinished("sess-1", "tu-2", "grep", "completed", 10, "partial")); err != nil {
		t.Fatal(err)
	}

	msgs := inner.snapshot()
	if len(msgs) < 2 {
		t.Fatalf("expected delta flush + completed, got %d", len(msgs))
	}
	if msgs[0].Data == nil || msgs[0].Data.Delta != "partial" {
		t.Fatalf("first message should be flushed delta, got %#v", msgs[0])
	}
}
