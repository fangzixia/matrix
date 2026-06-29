package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestLogLLMHTTPRequestWritesRawBody(t *testing.T) {
	var buf bytes.Buffer
	restore := SetLLMWriterForTest(&buf)
	defer restore()

	ctx := With(context.Background(), Fields{
		FieldSessionID: "sess-1",
		FieldComponent: "query",
		FieldTurn:      "2",
	})
	body := `{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`
	LogLLMHTTPRequest(ctx, LLMHTTPMeta{
		URL:       "http://localhost/v1/chat/completions",
		BaseURL:   "http://localhost",
		Model:     "gpt-test",
		ModelName: "Test Model",
	}, body)

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if rec["event"] != "llm.request" {
		t.Fatalf("event = %v", rec["event"])
	}
	if rec["request_body"] != body {
		t.Fatalf("request_body = %v", rec["request_body"])
	}
	if rec["session_id"] != "sess-1" || rec["component"] != "query" || rec["turn"] != "2" {
		t.Fatalf("context fields missing: %#v", rec)
	}
	if rec["model_name"] != "Test Model" {
		t.Fatalf("model_name = %v", rec["model_name"])
	}
}

func TestLogLLMHTTPResponseWritesRawBody(t *testing.T) {
	var buf bytes.Buffer
	restore := SetLLMWriterForTest(&buf)
	defer restore()

	LogLLMHTTPResponse(context.Background(), LLMHTTPMeta{
		URL:     "http://localhost/v1/chat/completions",
		BaseURL: "http://localhost",
		Model:   "gpt-test",
	}, 401, `{"error":"invalid key"}`, 12*time.Millisecond)

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if rec["event"] != "llm.response" {
		t.Fatalf("event = %v", rec["event"])
	}
	if rec["status_code"].(float64) != 401 {
		t.Fatalf("status_code = %v", rec["status_code"])
	}
	if rec["response_body"] != `{"error":"invalid key"}` {
		t.Fatalf("response_body = %v", rec["response_body"])
	}
	if rec["latency_ms"].(float64) != 12 {
		t.Fatalf("latency_ms = %v", rec["latency_ms"])
	}
}
