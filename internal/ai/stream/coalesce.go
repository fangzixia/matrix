package stream

import (
	"context"
	"sync"
	"time"
)

type CoalesceSink struct {
	inner     Sink
	sessionID string
	mu        sync.Mutex
	pending   map[string]string
	flushCh   chan struct{}
	done      chan struct{}
}

func NewCoalesceSink(inner Sink, sessionID string, interval time.Duration) *CoalesceSink {
	c := &CoalesceSink{
		inner:     inner,
		sessionID: sessionID,
		pending:   make(map[string]string),
		flushCh:   make(chan struct{}, 1),
		done:      make(chan struct{}),
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
	defer c.mu.Unlock()
	for key, text := range c.pending {
		if text == "" {
			delete(c.pending, key)
			continue
		}
		sid := c.sessionID
		var msg Message
		switch key {
		case DeltaText:
			msg = TextDelta(sid, text, 0)
		case DeltaThinking:
			msg = ThinkingDelta(sid, text, 0)
		default:
			delete(c.pending, key)
			continue
		}
		delete(c.pending, key)
		_ = c.inner.Publish(context.Background(), msg)
	}
}

func (c *CoalesceSink) Close() {
	close(c.done)
}

func (c *CoalesceSink) Publish(ctx context.Context, msg Message) error {
	if msg.SessionID != "" {
		c.sessionID = msg.SessionID
	}
	if msg.Type == TypeStreamEvent && msg.Event != nil &&
		msg.Event.Type == EventContentBlockDelta && msg.Event.Delta != nil {
		d := msg.Event.Delta
		key := d.Type
		var chunk string
		switch d.Type {
		case DeltaText:
			chunk = d.Text
		case DeltaThinking:
			chunk = d.Thinking
		default:
			return c.inner.Publish(ctx, msg)
		}
		if chunk == "" {
			return nil
		}
		c.mu.Lock()
		c.pending[key] += chunk
		c.mu.Unlock()
		select {
		case c.flushCh <- struct{}{}:
		default:
		}
		return nil
	}
	c.flush()
	return c.inner.Publish(ctx, msg)
}
