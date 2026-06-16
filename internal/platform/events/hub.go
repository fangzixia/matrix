package events

import (
	"context"
	"encoding/json"
	"sync"

	"matrix/internal/ai/stream"
)

const (
	EventAgentStream    = "agent:stream"
	EventSubAgentUpdate = "subagent:update"
	EventSubAgentDone   = "subagent:done"
)

type Subscriber chan stream.Message

type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[Subscriber]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[Subscriber]struct{})}
}

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

func (h *Hub) PublishNotification(userID string, payload any) {
	b, _ := json.Marshal(payload)
	h.Publish("user:"+userID, stream.Message{
		Type: "notification", SessionID: "user:" + userID, Output: string(b),
	})
}

func (h *Hub) Sink(runID string) stream.Sink {
	return stream.FuncSink(func(_ context.Context, msg stream.Message) error {
		if msg.SessionID == "" {
			msg.SessionID = runID
		}
		h.Publish(runID, msg)
		return nil
	})
}

func MarshalSSE(_ string, v any) ([]byte, error) {
	return json.Marshal(v)
}
