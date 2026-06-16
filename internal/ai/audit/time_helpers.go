package audit

import "time"

func formatTimeUTC(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func timeRFC3339After(a, b string) bool {
	return parseTimeRFC3339(a).After(parseTimeRFC3339(b))
}

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
