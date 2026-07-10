package audit

import "time"

// formatTimeUTC 将时间格式化为 UTC RFC3339 字符串。
func formatTimeUTC(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
