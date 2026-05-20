package llm

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestComplete_TextResponse(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"summary text"},"finish_reason":"stop"}]}`)
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
