package stream

import (
	"context"
	"sync"
	"time"

	agui "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

type pendingToolOutput struct {
	toolName string
	ev       Event
}

type OutputCoalesceSink struct {
	inner   Sink
	mu      sync.Mutex
	pending map[string]*pendingToolOutput
	flushCh chan struct{}
	done    chan struct{}
}

func NewOutputCoalesceSink(inner Sink, _ string, interval time.Duration) *OutputCoalesceSink {
	if interval <= 0 {
		interval = 200 * time.Millisecond
	}
	c := &OutputCoalesceSink{
		inner: inner, pending: make(map[string]*pendingToolOutput),
		flushCh: make(chan struct{}, 1), done: make(chan struct{}),
	}
	go c.loop(interval)
	return c
}

func (c *OutputCoalesceSink) loop(interval time.Duration) {
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

func (c *OutputCoalesceSink) flush() {
	c.mu.Lock()
	pending := c.pending
	c.pending = make(map[string]*pendingToolOutput)
	inner := c.inner
	c.mu.Unlock()
	if inner == nil {
		return
	}
	for _, p := range pending {
		if p != nil && p.ev != nil {
			_ = inner.Publish(context.Background(), p.ev)
		}
	}
}

func (c *OutputCoalesceSink) Close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

func (c *OutputCoalesceSink) Publish(ctx context.Context, ev Event) error {
	if IsToolOutputDelta(ev) {
		toolCallID, toolName, _, ok := ToolOutputDeltaFields(ev)
		if !ok {
			return nil
		}
		c.mu.Lock()
		p := c.pending[toolCallID]
		if p == nil {
			p = &pendingToolOutput{toolName: toolName, ev: ev}
			c.pending[toolCallID] = p
		} else {
			p.ev = MergeToolOutputDelta(p.ev, ev)
			if p.toolName == "" {
				p.toolName = toolName
			}
		}
		c.mu.Unlock()
		select {
		case c.flushCh <- struct{}{}:
		default:
		}
		return nil
	}
	if endToolID := toolEndID(ev); endToolID != "" {
		c.flushTool(endToolID)
	}
	c.flush()
	if c.inner == nil {
		return nil
	}
	return c.inner.Publish(ctx, ev)
}

func (c *OutputCoalesceSink) flushTool(toolCallID string) {
	c.mu.Lock()
	p, ok := c.pending[toolCallID]
	if ok {
		delete(c.pending, toolCallID)
	}
	inner := c.inner
	c.mu.Unlock()
	if !ok || p == nil || p.ev == nil || inner == nil {
		return
	}
	_ = inner.Publish(context.Background(), p.ev)
}

func toolEndID(ev Event) string {
	switch e := ev.(type) {
	case *agui.ToolCallEndEvent:
		return e.ToolCallID
	case *agui.ToolCallResultEvent:
		return e.ToolCallID
	default:
		return ""
	}
}
