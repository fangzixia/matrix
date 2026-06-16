package tools

import "context"

type toolCallIDKey struct{}

// ContextWithToolCallID 将 LLM tool_use id 写入 context，供 coordinator 等关联子 Agent 与父工具块。
func ContextWithToolCallID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, toolCallIDKey{}, id)
}

// ToolCallIDFromContext 读取 tool_use id；不存在时返回空字符串。
func ToolCallIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(toolCallIDKey{}).(string); ok {
		return v
	}
	return ""
}
