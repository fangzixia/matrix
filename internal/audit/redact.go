package audit

import (
	"regexp"
	"strings"
)

var (
	skKeyRe    = regexp.MustCompile(`sk-[A-Za-z0-9_-]{8,}`)
	bearerRe   = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._-]+`)
	apiKeyRe   = regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[:=]\s*\S+`)
	passwordRe = regexp.MustCompile(`(?i)(password|passwd|secret|token)\s*[:=]\s*\S+`)
)

const redactedPlaceholder = "[REDACTED]"

// RedactString masks common secret patterns in a single string.
func RedactString(s string) string {
	if s == "" {
		return s
	}
	out := s
	out = skKeyRe.ReplaceAllString(out, redactedPlaceholder)
	out = bearerRe.ReplaceAllString(out, redactedPlaceholder)
	out = apiKeyRe.ReplaceAllString(out, redactedPlaceholder)
	out = passwordRe.ReplaceAllString(out, redactedPlaceholder)
	return out
}

// RedactData recursively redacts string values in a map/slice tree.
func RedactData(v any) any {
	switch x := v.(type) {
	case string:
		return RedactString(x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = RedactData(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = RedactData(val)
		}
		return out
	default:
		return v
	}
}

// Preview truncates s to maxRunes Unicode runes for audit/log previews.
func Preview(s string, maxRunes int) string {
	if maxRunes <= 0 || s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return RedactString(s)
	}
	return RedactString(string(runes[:maxRunes]) + "…")
}

// PreviewKeys redacts known sensitive map keys entirely.
func PreviewKeys(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	sensitive := []string{"api_key", "apikey", "authorization", "password", "secret", "token"}
	out := make(map[string]any, len(data))
	for k, v := range data {
		kl := strings.ToLower(k)
		masked := false
		for _, s := range sensitive {
			if strings.Contains(kl, s) {
				out[k] = redactedPlaceholder
				masked = true
				break
			}
		}
		if !masked {
			out[k] = RedactData(v)
		}
	}
	return out
}
