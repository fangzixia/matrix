package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogLLMRequestRedactsSecrets(t *testing.T) {
	var buf bytes.Buffer
	restore := SetLLMWriterForTest(&buf)
	defer restore()

	LogLLMRequest(nil, "model", []map[string]any{
		{
			"role":    "user",
			"content": "hello",
			"api_key": "secret-key",
		},
	}, map[string]any{
		"Authorization": "Bearer token",
	}, 100)

	out := buf.String()
	if strings.Contains(out, "secret-key") || strings.Contains(out, "Bearer token") {
		t.Fatalf("LLM log leaked secret: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected redaction marker in %s", out)
	}
}

func TestLogLLMErrorRedactsRawBody(t *testing.T) {
	var buf bytes.Buffer
	restore := SetLLMWriterForTest(&buf)
	defer restore()

	LogLLMError(nil, nil, 401, `{"error":"Authorization: Bearer secret"}`, 0)

	out := buf.String()
	if strings.Contains(out, "Bearer secret") {
		t.Fatalf("LLM error log leaked body: %s", out)
	}
}
