package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sseResponse 拼接若干 SSE data 行并在末尾追加 [DONE]。
func sseResponse(events ...string) string {
	var sb strings.Builder
	for _, e := range events {
		sb.WriteString("data: ")
		sb.WriteString(e)
		sb.WriteString("\n\n")
	}
	sb.WriteString("data: [DONE]\n\n")
	return sb.String()
}

// newTestClient 创建一个指向测试服务器的 Client。
func newTestClient(handler http.HandlerFunc) (*Client, *httptest.Server) {
	srv := httptest.NewServer(handler)
	c := NewClient(srv.URL, "test-key")
	return c, srv
}

// TestStream_TextOnly 验证纯文本流式响应能正确累积。
func TestStream_TextOnly(t *testing.T) {
	body := sseResponse(
		`{"choices":[{"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"content":", world"},"finish_reason":null}]}`,
		`{"choices":[{"delta":{},"finish_reason":"stop"}]}`,
	)
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	})
	defer srv.Close()

	var deltas []string
	var turn *AssistantTurn

	for ev := range c.Stream(context.Background(), ChatRequest{Model: "test"}) {
		if ev.Err != nil {
			t.Fatalf("意外错误: %v", ev.Err)
		}
		if ev.TextDelta != "" {
			deltas = append(deltas, ev.TextDelta)
		}
		if ev.Turn != nil {
			turn = ev.Turn
		}
	}

	if got := strings.Join(deltas, ""); got != "Hello, world" {
		t.Errorf("文本增量累积错误: %q", got)
	}
	if turn == nil {
		t.Fatal("期望收到最终 turn")
	}
	if turn.Content != "Hello, world" {
		t.Errorf("turn.Content 错误: %q", turn.Content)
	}
	if turn.FinishReason != "stop" {
		t.Errorf("finish_reason 错误: %q", turn.FinishReason)
	}
}

// TestStream_ToolCalls 验证 tool_call 增量能跨多个 SSE 事件正确累积。
func TestStream_ToolCalls(t *testing.T) {
	body := sseResponse(
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read_file","arguments":""}}]},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":"}}]},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"go.mod\"}"}}]},"finish_reason":null}]}`,
		`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
	)
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	})
	defer srv.Close()

	var turn *AssistantTurn
	for ev := range c.Stream(context.Background(), ChatRequest{Model: "test"}) {
		if ev.Err != nil {
			t.Fatalf("意外错误: %v", ev.Err)
		}
		if ev.Turn != nil {
			turn = ev.Turn
		}
	}

	if turn == nil {
		t.Fatal("期望收到最终 turn")
	}
	if len(turn.ToolCalls) != 1 {
		t.Fatalf("期望 1 个工具调用，实际 %d", len(turn.ToolCalls))
	}
	tc := turn.ToolCalls[0]
	if tc.ID != "call_1" {
		t.Errorf("tool call id 错误: %q", tc.ID)
	}
	if tc.Function.Name != "read_file" {
		t.Errorf("tool name 错误: %q", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"path":"go.mod"}` {
		t.Errorf("tool arguments 错误: %q", tc.Function.Arguments)
	}
	if turn.FinishReason != "tool_calls" {
		t.Errorf("finish_reason 错误: %q", turn.FinishReason)
	}
}

// TestStream_ServerError 验证 HTTP 非 200 响应触发错误事件。
func TestStream_ServerError(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"invalid api key"}}`)
	})
	defer srv.Close()

	var gotErr error
	for ev := range c.Stream(context.Background(), ChatRequest{Model: "test"}) {
		if ev.Err != nil {
			gotErr = ev.Err
		}
	}
	if gotErr == nil {
		t.Fatal("期望 401 返回错误")
	}
	if !strings.Contains(gotErr.Error(), "401") {
		t.Errorf("错误信息应包含 401: %v", gotErr)
	}
}

// TestStream_ContextCancel 验证 context 取消时流正确中止。
func TestStream_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		t.Error("context 已取消，服务端不应被调用")
	})
	defer srv.Close()

	var gotErr error
	for ev := range c.Stream(ctx, ChatRequest{Model: "test"}) {
		if ev.Err != nil {
			gotErr = ev.Err
		}
	}
	if gotErr == nil {
		t.Fatal("期望 context 取消错误")
	}
}
