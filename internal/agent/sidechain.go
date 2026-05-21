package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SidechainWriter 将子 Agent 流式事件追加到 JSONL 旁路 transcript。
type SidechainWriter struct {
	mu   sync.Mutex
	root string
}

// NewSidechainWriter 在 root 下创建 subagents 目录；root 为空则禁用持久化。
func NewSidechainWriter(root string) *SidechainWriter {
	if root == "" {
		return &SidechainWriter{}
	}
	_ = os.MkdirAll(filepath.Join(root, "subagents"), 0o755)
	return &SidechainWriter{root: root}
}

func (w *SidechainWriter) path(id ID) string {
	if w == nil || w.root == "" {
		return ""
	}
	return filepath.Join(w.root, "subagents", string(id)+".jsonl")
}

// Path 返回某 Agent 的 sidechain 文件路径。
func (w *SidechainWriter) Path(id ID) string {
	return w.path(id)
}

// Append 追加一行 JSON 记录；失败时静默（不阻塞 Agent）。
func (w *SidechainWriter) Append(id ID, record any) {
	if w == nil || w.root == "" {
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

// ReadTail 读取 sidechain 末尾最多 maxLines 行（用于 UI 展开）。
func (w *SidechainWriter) ReadTail(id ID, maxLines int) (string, error) {
	p := w.path(id)
	if p == "" {
		return "", fmt.Errorf("sidechain 未配置")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if maxLines <= 0 {
		return string(data), nil
	}
	lines := splitLines(string(data))
	if len(lines) <= maxLines {
		return string(data), nil
	}
	return joinLines(lines[len(lines)-maxLines:]), nil
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	b := lines[0]
	for i := 1; i < len(lines); i++ {
		b += "\n" + lines[i]
	}
	return b
}
