package audit

import "time"

// formatTimeUTC 将时间格式化为 UTC RFC3339 字符串。
func formatTimeUTC(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// timeRFC3339After 比较两个 RFC3339 时间字符串的先后。
func timeRFC3339After(a, b string) bool {
	return parseTimeRFC3339(a).After(parseTimeRFC3339(b))
}

// parseTimeRFC3339 解析 RFC3339 时间字符串。
func parseTimeRFC3339(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
