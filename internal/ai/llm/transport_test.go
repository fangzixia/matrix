package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"matrix/internal/platform/logging"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	client := NewClient(srv.URL, "")
	client.HTTPClient = &http.Client{
		Timeout:   client.HTTPClient.Timeout,
		Transport: newLoggingTransport(srv.Client().Transport, client),
	}
	return client
}

func countLLMEvents(logs, event string) int {
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		var rec map[string]any
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		if rec["event"] == event {
			n++
		}
	}
	return n
}

func TestLoggingTransportNon200LogsResponseAndRestoresBody(t *testing.T) {
	var buf bytes.Buffer
	restore := logging.SetLLMWriterForTest(&buf)
	defer restore()

	const errBody = `{"error":"invalid key"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(errBody))
	}))
	defer srv.Close()

	client := testClient(t, srv)
	reqBody := []byte(`{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`)
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		srv.URL+"/v1/chat/completions",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != errBody {
		t.Fatalf("body = %q, want %q", got, errBody)
	}

	logs := buf.String()
	if !strings.Contains(logs, `"event":"llm.request"`) {
		t.Fatalf("missing llm.request: %s", logs)
	}
	if !strings.Contains(logs, `"event":"llm.response"`) {
		t.Fatalf("missing llm.response: %s", logs)
	}
	if !strings.Contains(logs, "invalid key") {
		t.Fatalf("response body not logged: %s", logs)
	}
	if strings.Contains(logs, `"status_code":200`) {
		t.Fatalf("unexpected 200 status in logs: %s", logs)
	}
	if countLLMEvents(logs, "llm.response") != 1 {
		t.Fatalf("want exactly 1 llm.response, got %d: %s", countLLMEvents(logs, "llm.response"), logs)
	}
}

func TestLoggingTransport200StreamLogsOnClose(t *testing.T) {
	var buf bytes.Buffer
	restore := logging.SetLLMWriterForTest(&buf)
	defer restore()

	const sse = "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sse)
	}))
	defer srv.Close()

	client := testClient(t, srv)
	ch := client.Stream(context.Background(), ChatRequest{
		Model:    "test-model",
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
	if !strings.Contains(logs, `"status_code":200`) {
		t.Fatalf("missing status_code 200: %s", logs)
	}
	if !strings.Contains(logs, "data: [DONE]") {
		t.Fatalf("SSE body not logged: %s", logs)
	}
	if countLLMEvents(logs, "llm.request") != 1 {
		t.Fatalf("want 1 llm.request, got %d", countLLMEvents(logs, "llm.request"))
	}
	if countLLMEvents(logs, "llm.response") != 1 {
		t.Fatalf("want exactly 1 llm.response, got %d: %s", countLLMEvents(logs, "llm.response"), logs)
	}
}

func TestLoggingTransport200MultiChunkOneResponse(t *testing.T) {
	var buf bytes.Buffer
	restore := logging.SetLLMWriterForTest(&buf)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			`data: {"choices":[{"delta":{"content":"a"}}]}` + "\n\n",
			`data: {"choices":[{"delta":{"content":"b"}}]}` + "\n\n",
			`data: {"choices":[{"delta":{"content":"c"}}]}` + "\n\n",
			"data: [DONE]\n\n",
		}
		for _, c := range chunks {
			_, _ = io.WriteString(w, c)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer srv.Close()

	client := testClient(t, srv)
	ch := client.Stream(context.Background(), ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	var content string
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("unexpected error: %v", ev.Err)
		}
		if ev.Turn != nil {
			content = ev.Turn.Content
		}
	}
	if content != "abc" {
		t.Fatalf("content = %q", content)
	}

	logs := buf.String()
	if countLLMEvents(logs, "llm.response") != 1 {
		t.Fatalf("want exactly 1 llm.response for multi-chunk stream, got %d: %s", countLLMEvents(logs, "llm.response"), logs)
	}
	if !strings.Contains(logs, `"status_code":200`) {
		t.Fatalf("missing status 200: %s", logs)
	}
}

func TestLoggingTransportNon200Status500LogsOneResponse(t *testing.T) {
	var buf bytes.Buffer
	restore := logging.SetLLMWriterForTest(&buf)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := testClient(t, srv)
	ch := client.Stream(context.Background(), ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	var gotErr bool
	for ev := range ch {
		if ev.Err != nil {
			gotErr = true
		}
	}
	if !gotErr {
		t.Fatal("expected error")
	}

	logs := buf.String()
	if countLLMEvents(logs, "llm.request") != 1 {
		t.Fatalf("want 1 llm.request, got %d", countLLMEvents(logs, "llm.request"))
	}
	if countLLMEvents(logs, "llm.response") != 1 {
		t.Fatalf("want 1 llm.response, got %d: %s", countLLMEvents(logs, "llm.response"), logs)
	}
	if !strings.Contains(logs, `"status_code":500`) {
		t.Fatalf("missing status 500: %s", logs)
	}
	if !strings.Contains(logs, "internal error") {
		t.Fatalf("response body not logged: %s", logs)
	}
}

func TestClientStreamLogsRequestAndErrorOnHTTPFailure(t *testing.T) {
	var buf bytes.Buffer
	restore := logging.SetLLMWriterForTest(&buf)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer srv.Close()

	client := testClient(t, srv)
	ch := client.Stream(context.Background(), ChatRequest{
		Model:    "test-model",
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
	if !strings.Contains(logs, `"event":"llm.response"`) {
		t.Fatalf("missing llm.response: %s", logs)
	}
	if countLLMEvents(logs, "llm.response") != 1 {
		t.Fatalf("want 1 llm.response, got %d", countLLMEvents(logs, "llm.response"))
	}
}

func TestLoggingTransportDoesNotAlterSSEResult(t *testing.T) {
	const sse = "data: {\"choices\":[{\"delta\":{\"content\":\"parity\"}}]}\n\ndata: [DONE]\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sse)
	}))
	defer srv.Close()

	withLog := testClient(t, srv)
	plain := NewClient(srv.URL, "")
	plain.HTTPClient = srv.Client()

	req := ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	}

	var turnLogged, turnPlain *AssistantTurn
	for ev := range withLog.Stream(context.Background(), req) {
		if ev.Err != nil {
			t.Fatalf("logged client: %v", ev.Err)
		}
		if ev.Turn != nil {
			turnLogged = ev.Turn
		}
	}
	for ev := range plain.Stream(context.Background(), req) {
		if ev.Err != nil {
			t.Fatalf("plain client: %v", ev.Err)
		}
		if ev.Turn != nil {
			turnPlain = ev.Turn
		}
	}
	if turnLogged == nil || turnPlain == nil {
		t.Fatal("missing turn")
	}
	if turnLogged.Content != turnPlain.Content {
		t.Fatalf("content mismatch: logged=%q plain=%q", turnLogged.Content, turnPlain.Content)
	}
}

func TestLoggingTransportRedirectPreservesPOSTBody(t *testing.T) {
	var redirected bool
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.URL.RawQuery != "ok=1" {
			if redirected {
				t.Fatal("unexpected second redirect")
			}
			redirected = true
			http.Redirect(w, r, "/v1/chat/completions?ok=1", http.StatusTemporaryRedirect)
			return
		}
		got, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var partial struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		if err := json.Unmarshal(got, &partial); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		if partial.Model != "test-model" || !partial.Stream {
			t.Fatalf("body after redirect = %s", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL, "")
	client.HTTPClient = &http.Client{
		Timeout:   client.HTTPClient.Timeout,
		Transport: newLoggingTransport(srv.Client().Transport, client),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 2 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	for ev := range client.Stream(context.Background(), ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	}) {
		if ev.Err != nil {
			t.Fatalf("stream error: %v", ev.Err)
		}
	}
	if !redirected {
		t.Fatal("redirect was not followed")
	}
}

func TestClientStreamContextFieldsInLogs(t *testing.T) {
	var buf bytes.Buffer
	restore := logging.SetLLMWriterForTest(&buf)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	ctx := logging.With(context.Background(), logging.Fields{
		logging.FieldSessionID: "run-abc",
		logging.FieldComponent: "query",
		logging.FieldTurn:      "3",
	})
	client := testClient(t, srv)
	client.ModelName = "Display Name"
	for ev := range client.Stream(ctx, ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	}) {
		if ev.Err != nil {
			t.Fatalf("unexpected error: %v", ev.Err)
		}
	}

	var found bool
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var rec map[string]any
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		if rec["event"] != "llm.request" {
			continue
		}
		found = true
		if rec["session_id"] != "run-abc" || rec["component"] != "query" || rec["turn"] != "3" {
			t.Fatalf("context fields: %#v", rec)
		}
		if rec["model_name"] != "Display Name" {
			t.Fatalf("model_name = %v", rec["model_name"])
		}
	}
	if !found {
		t.Fatalf("missing llm.request with context: %s", buf.String())
	}
}
