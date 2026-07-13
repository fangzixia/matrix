package query

import "unicode/utf8"

// PreviewText 将 s 截断至 maxRunes 个 Unicode 字符，用于审计/日志预览。
func PreviewText(s string, maxRunes int) string {
	if maxRunes <= 0 || s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}

// TruncateRunesCount 返回 s 的 rune 数（供测试或估算）。
func TruncateRunesCount(s string) int {
	return utf8.RuneCountInString(s)
}
