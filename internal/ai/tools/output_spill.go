package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const maxToolOutputSpillBytes = 5 * 1024 * 1024 // 5MB

// OutputSpillWriter 将工具输出追加写入沙箱内磁盘文件。
type OutputSpillWriter struct {
	path    string
	mu      sync.Mutex
	written int64
	capped  bool
}

// NewOutputSpillWriter 创建 toolUseID 对应的 spill 文件写入器。
func NewOutputSpillWriter(ctx context.Context, toolUseID string) (*OutputSpillWriter, error) {
	root := SandboxFrom(ctx)
	if root == "" {
		return nil, fmt.Errorf("沙箱未配置")
	}
	dir := filepath.Join(root, ".matrix", "tool-outputs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, sanitizeToolUseID(toolUseID)+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	_ = f.Close()
	return &OutputSpillWriter{path: path}, nil
}

// Path 返回 spill 文件路径。
func (w *OutputSpillWriter) Path() string {
	if w == nil {
		return ""
	}
	return w.path
}

// Append 追加文本；超出上限后写入截断标记并忽略后续内容。
func (w *OutputSpillWriter) Append(chunk string) error {
	if w == nil || chunk == "" {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.capped {
		return nil
	}
	n := int64(len(chunk))
	if w.written+n > maxToolOutputSpillBytes {
		w.capped = true
		trunc := "\n[output truncated: exceeded 5MB disk cap]\n"
		f, err := os.OpenFile(w.path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, err = f.WriteString(trunc)
		f.Close()
		if err != nil {
			return err
		}
		w.written += int64(len(trunc))
		return nil
	}
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(chunk); err != nil {
		return err
	}
	w.written += n
	return nil
}

func sanitizeToolUseID(id string) string {
	out := make([]byte, 0, len(id))
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "tool"
	}
	return string(out)
}
