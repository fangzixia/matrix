package desktop

import (
	"context"
	"sync"
	"time"

	"matrix/internal/stream"
)

// coalesceSink 合并高频 text/thinking delta，降低 Wails 事件压力。
type coalesceSink struct {
	inner     stream.Sink
	sessionID string
	mu        sync.Mutex
	pending   map[string]string
	flushCh   chan struct{}
	done      chan struct{}
}

func newCoalesceSink(inner stream.Sink, sessionID string, interval time.Duration) *coalesceSink {
	c := &coalesceSink{
		inner:     inner,
		sessionID: sessionID,
		pending:   make(map[string]string),
		flushCh:   make(chan struct{}, 1),
		done:      make(chan struct{}),
	}
	go c.loop(interval)
	return c
}

func (c *coalesceSink) loop(interval time.Duration) {
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

func (c *coalesceSink) flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, text := range c.pending {
		if text == "" {
			delete(c.pending, key)
			continue
		}
		sid := c.sessionID
		var msg stream.Message
		switch key {
		case stream.DeltaText:
			msg = stream.TextDelta(sid, text, 0)
		case stream.DeltaThinking:
			msg = stream.ThinkingDelta(sid, text, 0)
		default:
			delete(c.pending, key)
			continue
		}
		delete(c.pending, key)
		_ = c.inner.Publish(context.Background(), msg)
	}
}

func (c *coalesceSink) close() {
	close(c.done)
}

func (c *coalesceSink) Publish(ctx context.Context, msg stream.Message) error {
	if msg.SessionID != "" {
		c.sessionID = msg.SessionID
	}
	if msg.Type == stream.TypeStreamEvent && msg.Event != nil &&
		msg.Event.Type == stream.EventContentBlockDelta && msg.Event.Delta != nil {
		d := msg.Event.Delta
		key := d.Type
		var chunk string
		switch d.Type {
		case stream.DeltaText:
			chunk = d.Text
		case stream.DeltaThinking:
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

// wailsSink 将消息推送到 Wails 前端。
type wailsSink struct {
	emit func(msg stream.Message)
}

func (w wailsSink) Publish(_ context.Context, msg stream.Message) error {
	if w.emit != nil {
		w.emit(msg)
	}
	return nil
}
