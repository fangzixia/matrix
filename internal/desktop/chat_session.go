package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"matrix/internal/query"
)

// ChatTurn 是自由对话前端传入的一轮摘要（仅 user/assistant 文本）。
type ChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatSessionRequest 自由对话多轮请求。
type ChatSessionRequest struct {
	ChatSessionID string     `json:"chatSessionId"`
	Message       string     `json:"message"`
	Bootstrap     []ChatTurn `json:"bootstrap,omitempty"`
}

type chatTranscriptStore struct {
	mu    sync.RWMutex
	cache map[string][]query.Message
}

func newChatTranscriptStore() *chatTranscriptStore {
	return &chatTranscriptStore{cache: make(map[string][]query.Message)}
}

func (s *chatTranscriptStore) get(id string) ([]query.Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs, ok := s.cache[id]
	if !ok || len(msgs) == 0 {
		return nil, false
	}
	out := make([]query.Message, len(msgs))
	copy(out, msgs)
	return out, true
}

func (s *chatTranscriptStore) set(id string, msgs []query.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]query.Message, len(msgs))
	copy(cp, msgs)
	s.cache[id] = cp
}

func (s *chatTranscriptStore) delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, id)
}

func chatTurnsToMessages(turns []ChatTurn) []query.Message {
	out := make([]query.Message, 0, len(turns))
	for _, t := range turns {
		role := query.RoleUser
		switch t.Role {
		case "assistant":
			role = query.RoleAssistant
		case "user", "":
			role = query.RoleUser
		default:
			continue
		}
		content := t.Content
		if content == "" {
			continue
		}
		out = append(out, query.Message{Role: role, Content: content})
	}
	return out
}

func persistedTranscriptPath(workspaceRoot, chatSessionID string) string {
	if workspaceRoot == "" || chatSessionID == "" {
		return ""
	}
	return filepath.Join(workspaceRoot, ".matrix", "chat-sessions", chatSessionID+".json")
}

func loadPersistedTranscript(path string) ([]query.Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var msgs []query.Message
	if err := json.Unmarshal(data, &msgs); err != nil {
		return nil, fmt.Errorf("parse chat transcript: %w", err)
	}
	return msgs, nil
}

func savePersistedTranscript(path string, msgs []query.Message) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func removePersistedTranscript(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path)
}

// loadChatTranscript 优先内存，其次磁盘，最后用前端 bootstrap 恢复 UI 历史。
func (b *Bridge) loadChatTranscript(chatSessionID string, bootstrap []ChatTurn) ([]query.Message, error) {
	if chatSessionID == "" {
		return nil, fmt.Errorf("chatSessionId 不能为空")
	}
	if msgs, ok := b.chatStore.get(chatSessionID); ok {
		return msgs, nil
	}
	path := persistedTranscriptPath(b.workspaceRoot(), chatSessionID)
	if path != "" {
		msgs, err := loadPersistedTranscript(path)
		if err != nil {
			return nil, err
		}
		if len(msgs) > 0 {
			b.chatStore.set(chatSessionID, msgs)
			return msgs, nil
		}
	}
	if len(bootstrap) > 0 {
		msgs := chatTurnsToMessages(bootstrap)
		b.chatStore.set(chatSessionID, msgs)
		return msgs, nil
	}
	return nil, nil
}

func (b *Bridge) saveChatTranscript(chatSessionID string, msgs []query.Message) error {
	if chatSessionID == "" {
		return nil
	}
	b.chatStore.set(chatSessionID, msgs)
	path := persistedTranscriptPath(b.workspaceRoot(), chatSessionID)
	if path == "" {
		return nil
	}
	return savePersistedTranscript(path, msgs)
}

// ClearChatSession 删除某聊天的 Agent 上下文（内存 + 磁盘）。
func (b *Bridge) ClearChatSession(chatSessionID string) error {
	if chatSessionID == "" {
		return nil
	}
	b.chatStore.delete(chatSessionID)
	removePersistedTranscript(persistedTranscriptPath(b.workspaceRoot(), chatSessionID))
	return nil
}
