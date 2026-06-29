// Package docmeta 提供 docs 下 Markdown 文档元数据提取 helper。
package docmeta

import "strings"

// TitleOrFallback 从 Markdown 内容提取一级标题；没有标题时返回 fallback。
func TitleOrFallback(fallback, content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return fallback
}
