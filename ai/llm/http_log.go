package llm

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// HTTPMeta 描述单次 LLM HTTP 往返的元数据。
type HTTPMeta struct {
	URL       string
	BaseURL   string
	Model     string
	ModelName string
	SessionID string
	RunID     string
	RequestID string
}

var (
	httpHookMu  sync.RWMutex
	httpReqHook func(context.Context, HTTPMeta, string)
	httpResHook func(context.Context, HTTPMeta, int, string, time.Duration)
)

// SetHTTPLogHooks 注册 LLM HTTP 原始请求/响应旁路（宿主可写入 llm.log）。
func SetHTTPLogHooks(req func(context.Context, HTTPMeta, string), res func(context.Context, HTTPMeta, int, string, time.Duration)) {
	httpHookMu.Lock()
	defer httpHookMu.Unlock()
	httpReqHook = req
	httpResHook = res
}

func logHTTPRequest(ctx context.Context, meta HTTPMeta, requestBody string) {
	httpHookMu.RLock()
	fn := httpReqHook
	httpHookMu.RUnlock()
	if fn != nil {
		fn(ctx, meta, requestBody)
		return
	}
	slog.DebugContext(ctx, "llm.request", "url", meta.URL, "model", meta.Model)
}

func logHTTPResponse(ctx context.Context, meta HTTPMeta, statusCode int, responseBody string, latency time.Duration) {
	httpHookMu.RLock()
	fn := httpResHook
	httpHookMu.RUnlock()
	if fn != nil {
		fn(ctx, meta, statusCode, responseBody, latency)
		return
	}
	slog.DebugContext(ctx, "llm.response", "url", meta.URL, "status", statusCode, "latency_ms", latency.Milliseconds())
}

func logClientError(ctx context.Context, model, modelName, baseURL, url string, err error) error {
	slog.WarnContext(ctx, "llm: 客户端错误", "model", model, "model_name", modelName, "base_url", baseURL, "url", url, "error", err)
	return err
}
