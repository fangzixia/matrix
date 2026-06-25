package stream

import (
	"context"
	"sync"
	"time"
)

// OutputCoalesceSink 合并短时间内的 tool_output_delta 再转发到内层 Sink。
type OutputCoalesceSink struct {
	inner     Sink
	sessionID string
	mu        sync.Mutex
	pending   map[string]*ToolProgressData // key: tool_use_id
	flushCh   chan struct{}
	done      chan struct{}
}

// NewOutputCoalesceSink 创建按 interval 批量 flush 工具输出 delta 的 Sink。
func NewOutputCoalesceSink(inner Sink, sessionID string, interval time.Duration) *OutputCoalesceSink {
	c := &OutputCoalesceSink{
		inner:     inner,
		sessionID: sessionID,
		pending:   make(map[string]*ToolProgressData),
		flushCh:   make(chan struct{}, 1),
		done:      make(chan struct{}),
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
	c.pending = make(map[string]*ToolProgressData)
	sid := c.sessionID
	c.mu.Unlock()

	for toolUseID, data := range pending {
		if data == nil || data.Delta == "" {
			continue
		}
		msg := ToolOutputDelta(sid, toolUseID, data.ToolName, data.Delta, data.OutputOffset)
		_ = c.inner.Publish(context.Background(), msg)
	}
}

// Close 停止 flush 循环并写出剩余 pending delta。
func (c *OutputCoalesceSink) Close() {
	close(c.done)
}

// Publish 合并 tool_output_delta 或直通其它消息类型。
func (c *OutputCoalesceSink) Publish(ctx context.Context, msg Message) error {
	if msg.SessionID != "" {
		c.sessionID = msg.SessionID
	}
	if msg.Type == TypeProgress && msg.Data != nil && msg.Data.Type == DataToolOutputDelta {
		if msg.ToolUseID == "" || msg.Data.Delta == "" {
			return nil
		}
		c.mu.Lock()
		p := c.pending[msg.ToolUseID]
		if p == nil {
			p = &ToolProgressData{
				Type:     DataToolOutputDelta,
				Status:   "streaming",
				ToolName: msg.Data.ToolName,
			}
			c.pending[msg.ToolUseID] = p
		}
		p.Delta += msg.Data.Delta
		if msg.Data.OutputOffset > p.OutputOffset {
			p.OutputOffset = msg.Data.OutputOffset
		}
		c.mu.Unlock()
		select {
		case c.flushCh <- struct{}{}:
		default:
		}
		return nil
	}
	// 工具结束等事件前先 flush 该 tool 的 pending delta
	if msg.Type == TypeProgress && msg.Data != nil &&
		msg.Data.Type == DataToolProgress &&
		(msg.Data.Status == "completed" || msg.Data.Status == "failed") &&
		msg.ToolUseID != "" {
		c.flushTool(msg.ToolUseID)
	}
	c.flush()
	return c.inner.Publish(ctx, msg)
}

func (c *OutputCoalesceSink) flushTool(toolUseID string) {
	c.mu.Lock()
	data, ok := c.pending[toolUseID]
	if ok {
		delete(c.pending, toolUseID)
	}
	sid := c.sessionID
	c.mu.Unlock()
	if !ok || data == nil || data.Delta == "" {
		return
	}
	msg := ToolOutputDelta(sid, toolUseID, data.ToolName, data.Delta, data.OutputOffset)
	_ = c.inner.Publish(context.Background(), msg)
}
