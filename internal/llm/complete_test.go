package llm

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestComplete_TextResponse(t *testing.T) {
	body := sseResponse(
		`{"choices":[{"delta":{"role":"assistant","content":"summary "},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"content":"text"},"finish_reason":null}]}`,
		`{"choices":[{"delta":{},"finish_reason":"stop"}]}`,
	)
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	})
	defer srv.Close()

	got, err := c.Complete(context.Background(), ChatRequest{
		Model: "test",
		Messages: []ChatMessage{
			{Role: "user", Content: "summarize"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "summary text" {
		t.Fatalf("got %q", got)
	}
}
