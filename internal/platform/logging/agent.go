package logging

import (
	"context"
)

var agentLines *jsonLineWriter

// Agent 写入 agent 执行日志（JSON Lines）。
func Agent(msg string, args ...any) {
	writeAgent(nil, msg, args...)
}

// AgentCtx 写入 agent 执行日志，并合并 context 中的 Fields。
func AgentCtx(ctx context.Context, msg string, args ...any) {
	writeAgent(ctx, msg, args...)
}

func writeAgent(ctx context.Context, msg string, args ...any) {
	if agentLines == nil {
		return
	}
	record := map[string]any{"msg": msg}
	if ctx != nil {
		record = mergeMaps(ctxFieldsMap(ctx), record)
	}
	record = mergeMaps(record, argsToMap(args...))
	agentLines.writeRecord(record)
}
