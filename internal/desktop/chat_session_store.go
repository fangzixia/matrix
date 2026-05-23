package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"

	"matrix/internal/matrixpaths"
)

// ChatSessionStore 持久化对话历史列表（chat-history.json）。
type ChatSessionStore struct {
	rootFn func() string
}

// NewChatSessionStore 创建对话历史存储；rootFn 返回当前工作区绝对路径。
func NewChatSessionStore(rootFn func() string) *ChatSessionStore {
	return &ChatSessionStore{rootFn: rootFn}
}

// Load 读取当前工作区的对话历史列表。
func (s *ChatSessionStore) Load() ([]ChatSession, error) {
	path := matrixpaths.ChatHistoryFile(s.rootFn())
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sessions []ChatSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, err
	}
	if sessions == nil {
		return []ChatSession{}, nil
	}
	return sessions, nil
}

// Save 写入对话历史列表。
func (s *ChatSessionStore) Save(sessions []ChatSession) error {
	path := matrixpaths.ChatHistoryFile(s.rootFn())
	if path == "" {
		return nil
	}
	if sessions == nil {
		sessions = []ChatSession{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
