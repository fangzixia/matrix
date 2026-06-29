package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"matrix/internal/platform/logging"
)

func TestClientStreamLogsRequestAndErrorOnHTTPFailure(t *testing.T) {
	var buf bytes.Buffer
	restore := logging.SetLLMWriterForTest(&buf)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "bad-key")
	client.HTTPClient = srv.Client()

	ch := client.Stream(context.Background(), ChatRequest{
		Model:    "test",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	var finalErr error
	for ev := range ch {
		if ev.Err != nil {
			finalErr = ev.Err
		}
	}
	if finalErr == nil {
		t.Fatal("expected error event")
	}
	logs := buf.String()
	if !strings.Contains(logs, `"event":"llm.request"`) {
		t.Fatalf("missing llm.request: %s", logs)
	}
	if !strings.Contains(logs, `"event":"llm.error"`) {
		t.Fatalf("missing llm.error: %s", logs)
	}
	if strings.Contains(logs, `"event":"llm.response"`) {
		t.Fatalf("unexpected llm.response: %s", logs)
	}
}

func TestClientStreamLogsRequestAndResponseOnSuccess(t *testing.T) {
	var buf bytes.Buffer
	restore := logging.SetLLMWriterForTest(&buf)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, `data: {"choices":[{"delta":{"content":"hello"},"finish_reason":null}]}`+"\n")
		_, _ = io.WriteString(w, "data: [DONE]\n")
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "")
	client.HTTPClient = srv.Client()

	ch := client.Stream(context.Background(), ChatRequest{
		Model:    "test",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	var turn *AssistantTurn
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("unexpected error: %v", ev.Err)
		}
		if ev.Turn != nil {
			turn = ev.Turn
		}
	}
	if turn == nil || turn.Content != "hello" {
		t.Fatalf("unexpected turn: %#v", turn)
	}
	logs := buf.String()
	if !strings.Contains(logs, `"event":"llm.request"`) {
		t.Fatalf("missing llm.request: %s", logs)
	}
	if !strings.Contains(logs, `"event":"llm.response"`) {
		t.Fatalf("missing llm.response: %s", logs)
	}
}

func TestClientContextLogsErrorOnEmptyContent(t *testing.T) {
	var buf bytes.Buffer
	restore := logging.SetLLMWriterForTest(&buf)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n")
		_, _ = io.WriteString(w, "data: [DONE]\n")
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "")
	client.HTTPClient = srv.Client()

	_, err := client.Context(context.Background(), ChatRequest{
		Model:    "test",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	logs := buf.String()
	var events []string
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		var rec map[string]any
		if json.Unmarshal([]byte(line), &rec) == nil {
			if ev, ok := rec["event"].(string); ok {
				events = append(events, ev)
			}
		}
	}
	if len(events) < 2 {
		t.Fatalf("expected at least request+outcome, got %v logs: %s", events, logs)
	}
	if events[0] != "llm.request" {
		t.Fatalf("first event should be llm.request, got %v", events)
	}
	last := events[len(events)-1]
	if last != "llm.response" && last != "llm.error" {
		t.Fatalf("last event should be response or error, got %v", events)
	}
}
