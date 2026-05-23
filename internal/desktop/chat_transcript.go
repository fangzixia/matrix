package desktop

import (
	"fmt"
	"path/filepath"
	"sync"

	"matrix/internal/matrixpaths"
	"matrix/internal/query"
)

// ChatTranscriptStore 管理多轮 chat 的 Agent transcript（内存 cache + 磁盘 snapshot）。
// chatSessionID 为聊天线程键；与 sessionRunner 的单次 run sessionID 无关。
type ChatTranscriptStore struct {
	rootFn func() string
	mu     sync.RWMutex
	cache  map[string][]query.Message
}

// NewChatTranscriptStore 创建 transcript 存储；rootFn 返回当前工作区根路径（通常即 Bridge.workspaceRoot）。
func NewChatTranscriptStore(rootFn func() string) *ChatTranscriptStore {
	return &ChatTranscriptStore{
		rootFn: rootFn,
		cache:  make(map[string][]query.Message),
	}
}

func (s *ChatTranscriptStore) transcriptsDir() string {
	return matrixpaths.ChatTranscriptsDir(s.rootFn())
}

func (s *ChatTranscriptStore) transcriptPath(chatSessionID string) string {
	if chatSessionID == "" {
		return ""
	}
	dir := s.transcriptsDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, chatSessionID+".json")
}

// getCached 读取内存副本。若条目不存在或 len==0，视为未命中并回退磁盘/bootstrap：
// 历史上从未用空 slice 表示「已缓存的空会话」，空 map 键与「未加载」无法区分，故 len==0 与 !ok 同等处理。
func (s *ChatTranscriptStore) getCached(chatSessionID string) ([]query.Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs, ok := s.cache[chatSessionID]
	if !ok || len(msgs) == 0 {
		return nil, false
	}
	out := make([]query.Message, len(msgs))
	copy(out, msgs)
	return out, true
}

func (s *ChatTranscriptStore) setCache(chatSessionID string, msgs []query.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]query.Message, len(msgs))
	copy(cp, msgs)
	s.cache[chatSessionID] = cp
}

func (s *ChatTranscriptStore) deleteCache(chatSessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, chatSessionID)
}

// InvalidateCache 清空内存 cache（工作区切换后必须调用，避免读到其它工作区的 transcript）。
func (s *ChatTranscriptStore) InvalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string][]query.Message)
}

// Load 优先内存，其次磁盘，最后用 bootstrap（对话历史降级）恢复。
func (s *ChatTranscriptStore) Load(chatSessionID string, bootstrap []ChatTurn) ([]query.Message, error) {
	if chatSessionID == "" {
		return nil, fmt.Errorf("chatSessionId 不能为空")
	}
	if msgs, ok := s.getCached(chatSessionID); ok {
		return msgs, nil
	}
	path := s.transcriptPath(chatSessionID)
	if path != "" {
		msgs, err := readTranscriptFile(path)
		if err != nil {
			return nil, err
		}
		if len(msgs) > 0 {
			s.setCache(chatSessionID, msgs)
			return msgs, nil
		}
	}
	if len(bootstrap) > 0 {
		msgs := chatTurnsToMessages(bootstrap)
		s.setCache(chatSessionID, msgs)
		return msgs, nil
	}
	return nil, nil
}

// Save 更新内存 cache 并将完整 transcript 快照写入磁盘。
func (s *ChatTranscriptStore) Save(chatSessionID string, msgs []query.Message) error {
	if chatSessionID == "" {
		return nil
	}
	s.setCache(chatSessionID, msgs)
	path := s.transcriptPath(chatSessionID)
	if path == "" {
		return nil
	}
	return writeTranscriptFile(path, msgs)
}

// Clear 删除指定聊天的内存 cache 与磁盘 transcript 文件。
func (s *ChatTranscriptStore) Clear(chatSessionID string) error {
	if chatSessionID == "" {
		return nil
	}
	s.deleteCache(chatSessionID)
	return removeTranscriptFile(s.transcriptPath(chatSessionID))
}
