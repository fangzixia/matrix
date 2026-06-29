package logging

import (
	"context"
	"io"
	"time"
)

var llmLines *jsonLineWriter

// LLMHTTPMeta 是 LLM HTTP 拦截日志的端点与模型元数据。
type LLMHTTPMeta struct {
	URL       string
	BaseURL   string
	Model     string
	ModelName string
}

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

// LogLLMHTTPRequest 记录 LLM HTTP 请求（原始 body 直写）。
func LogLLMHTTPRequest(ctx context.Context, meta LLMHTTPMeta, requestBody string) {
	if llmLines == nil {
		return
	}
	record := map[string]any{
		"event":        "llm.request",
		"url":          meta.URL,
		"base_url":     meta.BaseURL,
		"model":        meta.Model,
		"request_body": requestBody,
	}
	if meta.ModelName != "" {
		record["model_name"] = meta.ModelName
	}
	if ctx != nil {
		record = mergeMaps(ctxFieldsMap(ctx), record)
	}
	llmLines.writeRecord(record)
}

// LogLLMHTTPResponse 记录 LLM HTTP 响应（原始 body 直写）。
func LogLLMHTTPResponse(ctx context.Context, meta LLMHTTPMeta, statusCode int, responseBody string, latency time.Duration) {
	if llmLines == nil {
		return
	}
	record := map[string]any{
		"event":         "llm.response",
		"url":           meta.URL,
		"base_url":      meta.BaseURL,
		"model":         meta.Model,
		"status_code":   statusCode,
		"response_body": responseBody,
		"latency_ms":    latency.Milliseconds(),
	}
	if meta.ModelName != "" {
		record["model_name"] = meta.ModelName
	}
	if ctx != nil {
		record = mergeMaps(ctxFieldsMap(ctx), record)
	}
	llmLines.writeRecord(record)
}
