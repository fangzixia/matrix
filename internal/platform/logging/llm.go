package logging

import (
	"context"
	"io"
	"time"
)

var llmLines *jsonLineWriter

// SetLLMWriterForTest 将 LLM 日志写入 w；返回的函数用于测试结束后恢复。
func SetLLMWriterForTest(w io.Writer) func() {
	prev := llmLines
	if w == nil {
		llmLines = nil
	} else {
		llmLines = newJSONLineWriter(w)
	}
	return func() { llmLines = prev }
}

// LogLLMRequest 记录 LLM 客户端请求（原始 payload，不脱敏）。
func LogLLMRequest(ctx context.Context, model string, messages, tools any, maxTokens int) {
	if llmLines == nil {
		return
	}
	record := map[string]any{
		"event":      "llm.request",
		"model":      model,
		"messages":   messages,
		"tools":      tools,
		"max_tokens": maxTokens,
	}
	if ctx != nil {
		record = mergeMaps(ctxFieldsMap(ctx), record)
	}
	llmLines.writeRecord(record)
}

// LogLLMResponse 记录 LLM 客户端成功响应（原始内容，不脱敏）。
func LogLLMResponse(ctx context.Context, content, thinking string, toolCalls any, finishReason string, latency time.Duration) {
	if llmLines == nil {
		return
	}
	record := map[string]any{
		"event":         "llm.response",
		"content":       content,
		"thinking":      thinking,
		"tool_calls":    toolCalls,
		"finish_reason": finishReason,
		"latency_ms":    latency.Milliseconds(),
	}
	if ctx != nil {
		record = mergeMaps(ctxFieldsMap(ctx), record)
	}
	llmLines.writeRecord(record)
}

// LogLLMError 记录 LLM 客户端错误（含 HTTP 非 200 的原始 body）。
func LogLLMError(ctx context.Context, err error, statusCode int, body string, latency time.Duration) {
	if llmLines == nil {
		return
	}
	record := map[string]any{
		"event":       "llm.error",
		"latency_ms":  latency.Milliseconds(),
		"status_code": statusCode,
		"body":        body,
	}
	if err != nil {
		record["error"] = err.Error()
	}
	if ctx != nil {
		record = mergeMaps(ctxFieldsMap(ctx), record)
	}
	llmLines.writeRecord(record)
}
