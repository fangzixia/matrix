package query

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"matrix/internal/llm"
)

func TestLLMCompactHistory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"用户要修登录 bug"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "key")
	msgs := make([]Message, 0, 12)
	for i := 0; i < 10; i++ {
		msgs = append(msgs, Message{Role: RoleUser, Content: strings.Repeat("x ", 500)})
		msgs = append(msgs, Message{Role: RoleAssistant, Content: strings.Repeat("y ", 500)})
	}
	msgs = append(msgs, Message{Role: RoleUser, Content: "latest"})

	cfg := Config{
		LLM:   client,
		Model: "test",
		ContextPolicy: ContextPolicy{
			KeepRecentMessages: 2,
		},
	}

	out, ok := llmCompactHistory(context.Background(), cfg, msgs, 0, "test")
	if !ok {
		t.Fatal("expected LLM compact ok")
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 messages (boundary + 2 tail), got %d", len(out))
	}
	if out[0].Role != RoleSystem || !strings.Contains(out[0].Content, "[compact_boundary]") {
		t.Fatalf("missing boundary: %q", out[0].Content)
	}
	if !strings.Contains(out[0].Content, "用户要修登录 bug") {
		t.Fatalf("missing summary: %q", out[0].Content)
	}
	if out[2].Content != "latest" {
		t.Fatalf("tail not preserved: %q", out[2].Content)
	}
}
