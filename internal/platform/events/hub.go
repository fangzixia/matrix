// Package events 提供用户通知的内存 SSE 事件总线。
package events

import (
	"encoding/json"
	"sync"
)

const (
	// EventNotification 是用户站内通知的 SSE 事件名。
	EventNotification = "notification"
)

// Notification 是站内通知 SSE 载荷。
type Notification struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

// Subscriber 是订阅通知事件的 buffered channel。
type Subscriber chan Notification

// Hub 按 userID 维护 SSE 订阅者集合。
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[Subscriber]struct{}
}

// NewHub 创建空的事件总线。
func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[Subscriber]struct{})}
}

// Subscribe 为 key（如 user:{id}）注册订阅者并返回消息 channel。
func (h *Hub) Subscribe(key string) Subscriber {
	ch := make(Subscriber, 64)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[key] == nil {
		h.subs[key] = make(map[Subscriber]struct{})
	}
	h.subs[key][ch] = struct{}{}
	return ch
}

// Unsubscribe 移除订阅者并关闭其 channel。
func (h *Hub) Unsubscribe(key string, ch Subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.subs[key]; m != nil {
		delete(m, ch)
		close(ch)
		if len(m) == 0 {
			delete(h.subs, key)
		}
	}
}

func (h *Hub) publish(key string, msg Notification) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs[key] {
		select {
		case ch <- msg:
		default:
		}
	}
}

// PublishNotification 向指定用户推送站内通知 SSE 消息。
func (h *Hub) PublishNotification(userID string, payload any) {
	b, _ := json.Marshal(payload)
	h.publish("user:"+userID, Notification{
		Type: "notification", Payload: string(b),
	})
}
