package logging

import (
	"context"
	"encoding/json"
	"io"
	"strings"
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

// LogLLMRequest 记录 LLM 客户端请求，默认脱敏密钥类字段并截断长文本。
func LogLLMRequest(ctx context.Context, model string, messages, tools any, maxTokens int) {
	if llmLines == nil {
		return
	}
	record := map[string]any{
		"event":      "llm.request",
		"model":      model,
		"messages":   redactLLMValue(messages),
		"tools":      redactLLMValue(tools),
		"max_tokens": maxTokens,
	}
	if ctx != nil {
		record = mergeMaps(ctxFieldsMap(ctx), record)
	}
	llmLines.writeRecord(record)
}

// LogLLMResponse 记录 LLM 客户端成功响应，保留排查所需内容并截断长文本。
func LogLLMResponse(ctx context.Context, content, thinking string, toolCalls any, finishReason string, latency time.Duration) {
	if llmLines == nil {
		return
	}
	record := map[string]any{
		"event":         "llm.response",
		"content":       truncateLogText(content, 8000),
		"thinking":      truncateLogText(thinking, 4000),
		"tool_calls":    redactLLMValue(toolCalls),
		"finish_reason": finishReason,
		"latency_ms":    latency.Milliseconds(),
	}
	if ctx != nil {
		record = mergeMaps(ctxFieldsMap(ctx), record)
	}
	llmLines.writeRecord(record)
}

// LogLLMError 记录 LLM 客户端错误，HTTP body 会先脱敏并截断。
func LogLLMError(ctx context.Context, err error, statusCode int, body string, latency time.Duration) {
	if llmLines == nil {
		return
	}
	record := map[string]any{
		"event":       "llm.error",
		"latency_ms":  latency.Milliseconds(),
		"status_code": statusCode,
		"body":        truncateLogText(redactSecretText(body), 4000),
	}
	if err != nil {
		record["error"] = truncateLogText(redactSecretText(err.Error()), 2000)
	}
	if ctx != nil {
		record = mergeMaps(ctxFieldsMap(ctx), record)
	}
	llmLines.writeRecord(record)
}

func redactLLMValue(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		return truncateLogText(redactSecretText(x), 8000)
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = redactLLMValue(x[i])
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			if isSecretLogKey(k) {
				out[k] = "[REDACTED]"
				continue
			}
			out[k] = redactLLMValue(v)
		}
		return out
	case bool, int, int64, float64:
		return v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "[UNSERIALIZABLE]"
		}
		var decoded any
		if err := json.Unmarshal(b, &decoded); err != nil {
			return "[UNSERIALIZABLE]"
		}
		return redactLLMValue(decoded)
	}
}

func isSecretLogKey(key string) bool {
	k := strings.ToLower(key)
	return strings.Contains(k, "authorization") ||
		strings.Contains(k, "api_key") ||
		strings.Contains(k, "apikey") ||
		strings.Contains(k, "token") ||
		strings.Contains(k, "secret") ||
		strings.Contains(k, "password") ||
		strings.Contains(k, "credential")
}

func redactSecretText(s string) string {
	for _, marker := range []string{"Authorization:", "authorization:", "Bearer ", "api_key=", "api-key="} {
		if strings.Contains(s, marker) {
			return "[REDACTED]"
		}
	}
	return s
}

func truncateLogText(s string, max int) string {
	if max <= 0 || len([]rune(s)) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "\n[TRUNCATED]"
}
