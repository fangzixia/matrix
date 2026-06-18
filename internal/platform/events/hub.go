// Package events 提供 Run 与用户通知的内存 SSE 事件总线。
package events

import (
	"context"
	"encoding/json"
	"sync"

	"matrix/internal/ai/stream"
)

const (
	// EventAgentStream 是 Run 流式 Agent 输出的 SSE 事件名。
	EventAgentStream = "agent:stream"
	// EventSubAgentUpdate 是子 Agent 状态更新的 SSE 事件名。
	EventSubAgentUpdate = "subagent:update"
	// EventSubAgentDone 是子 Agent 结束的 SSE 事件名。
	EventSubAgentDone = "subagent:done"
	// EventNotification 是用户站内通知的 SSE 事件名。
	EventNotification = "notification"
)

// Subscriber 是订阅 Run 事件的 buffered channel。
type Subscriber chan stream.Message

// Hub 按 runID 维护 SSE 订阅者集合。
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[Subscriber]struct{}
}

// NewHub 创建空的事件总线。
func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[Subscriber]struct{})}
}

// Subscribe 为 runID 注册订阅者并返回消息 channel。
func (h *Hub) Subscribe(runID string) Subscriber {
	ch := make(Subscriber, 64)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[runID] == nil {
		h.subs[runID] = make(map[Subscriber]struct{})
	}
	h.subs[runID][ch] = struct{}{}
	return ch
}

// Unsubscribe 移除订阅者并关闭其 channel。
func (h *Hub) Unsubscribe(runID string, ch Subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.subs[runID]; m != nil {
		delete(m, ch)
		close(ch)
		if len(m) == 0 {
			delete(h.subs, runID)
		}
	}
}

// Publish 向 runID 的全部订阅者非阻塞推送消息。
func (h *Hub) Publish(runID string, msg stream.Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs[runID] {
		select {
		case ch <- msg:
		default:
		}
	}
}

// PublishNotification 向指定用户推送站内通知 SSE 消息。
func (h *Hub) PublishNotification(userID string, payload any) {
	b, _ := json.Marshal(payload)
	h.Publish("user:"+userID, stream.Message{
		Type: "notification", SessionID: "user:" + userID, Output: string(b),
	})
}

// Sink 返回将 stream.Message 转发到 Hub 的 stream.Sink。
func (h *Hub) Sink(runID string) stream.Sink {
	return stream.FuncSink(func(_ context.Context, msg stream.Message) error {
		if msg.SessionID == "" {
			msg.SessionID = runID
		}
		h.Publish(runID, msg)
		return nil
	})
}

// MarshalSSE 将 payload 序列化为 SSE data 字段 JSON。
func MarshalSSE(_ string, v any) ([]byte, error) {
	return json.Marshal(v)
}
