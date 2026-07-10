package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// SidechainWriter 将子 Agent 流式事件追加到 subagents 目录下的 JSONL 旁路 transcript。
type SidechainWriter struct {
	mu  sync.Mutex
	dir string // subagents 目录绝对路径
}

// NewSidechainWriter 在 subagentsDir 下写入 sidechain；空路径则禁用持久化。
func NewSidechainWriter(subagentsDir string) *SidechainWriter {
	if subagentsDir == "" {
		return &SidechainWriter{}
	}
	_ = os.MkdirAll(subagentsDir, 0o755)
	return &SidechainWriter{dir: subagentsDir}
}

// path 返回 sidechain 文件路径。
func (w *SidechainWriter) path(id ID) string {
	if w == nil || w.dir == "" {
		return ""
	}
	return filepath.Join(w.dir, string(id)+".jsonl")
}

// Path 返回某 Agent 的 sidechain 文件路径。
func (w *SidechainWriter) Path(id ID) string {
	return w.path(id)
}

// Append 追加一行 JSON 记录；失败时静默（不阻塞 Agent）。
func (w *SidechainWriter) Append(id ID, record any) {
	if w == nil || w.dir == "" {
		return
	}
	p := w.path(id)
	w.mu.Lock()
	defer w.mu.Unlock()
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	_ = enc.Encode(record)
}
