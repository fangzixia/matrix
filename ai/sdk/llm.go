package sdk

import "matrix/ai/llm"

// Client 为 OpenAI 兼容的 LLM 客户端。
type Client = llm.Client

// HTTPMeta 为 LLM HTTP 旁路日志元数据。
type HTTPMeta = llm.HTTPMeta

var (
	// NewClient 创建 LLM 客户端。
	NewClient = llm.NewClient
	// SetHTTPLogHooks 设置 LLM HTTP 请求/响应旁路日志钩子。
	SetHTTPLogHooks = llm.SetHTTPLogHooks
)
