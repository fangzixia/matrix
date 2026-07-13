package stream

import (
	"context"
	"sync"
	"time"

	agui "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

type CoalesceSink struct {
	inner   Sink
	mu      sync.Mutex
	pending map[string]Event
	flushCh chan struct{}
	done    chan struct{}
}

func NewCoalesceSink(inner Sink, _ string, interval time.Duration) *CoalesceSink {
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	c := &CoalesceSink{
		inner: inner, pending: make(map[string]Event),
		flushCh: make(chan struct{}, 1), done: make(chan struct{}),
	}
	go c.loop(interval)
	return c
}

func (c *CoalesceSink) loop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			c.flush()
			return
		case <-ticker.C:
			c.flush()
		case <-c.flushCh:
			c.flush()
		}
	}
}

func (c *CoalesceSink) flush() {
	c.mu.Lock()
	pending := c.pending
	c.pending = make(map[string]Event)
	inner := c.inner
	c.mu.Unlock()
	if inner == nil {
		return
	}
	for _, ev := range pending {
		if ev != nil {
			_ = inner.Publish(context.Background(), ev)
		}
	}
}

func (c *CoalesceSink) Close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

func (c *CoalesceSink) Publish(ctx context.Context, ev Event) error {
	if !IsTextContent(ev) {
		c.flush()
		if c.inner == nil {
			return nil
		}
		return c.inner.Publish(ctx, ev)
	}
	key := textCoalesceKey(ev)
	if key == "" {
		if c.inner == nil {
			return nil
		}
		return c.inner.Publish(ctx, ev)
	}
	c.mu.Lock()
	if prev, ok := c.pending[key]; ok {
		c.pending[key] = AppendTextDelta(prev, textCoalesceChunk(ev))
	} else {
		c.pending[key] = ev
	}
	c.mu.Unlock()
	select {
	case c.flushCh <- struct{}{}:
	default:
	}
	return nil
}

func textCoalesceKey(ev Event) string {
	switch e := ev.(type) {
	case *agui.TextMessageContentEvent:
		return "text:" + e.MessageID
	case *agui.ReasoningMessageContentEvent:
		return "reason:" + e.MessageID
	default:
		return ""
	}
}

func textCoalesceChunk(ev Event) string {
	switch e := ev.(type) {
	case *agui.TextMessageContentEvent:
		return e.Delta
	case *agui.ReasoningMessageContentEvent:
		return e.Delta
	default:
		return ""
	}
}
