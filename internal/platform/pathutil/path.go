// Package pathutil 提供跨模块复用的路径校验辅助函数。
package pathutil

import (
	"path/filepath"
	"strings"
)

// WithinRoot 报告 absPath 是否位于 root 目录内（含 root 自身）。
func WithinRoot(absPath, root string) bool {
	absPath = filepath.Clean(absPath)
	root = filepath.Clean(root)
	if absPath == root {
		return true
	}
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
