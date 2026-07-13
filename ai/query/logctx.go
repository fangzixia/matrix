package query

import (
	"context"
	"log/slog"
)

type logFieldsKey struct{}

// LogFields 为结构化 agent 日志附加上下文字段。
type LogFields map[string]string

const (
	logFieldSessionID = "session_id"
	logFieldTurn      = "turn"
	logFieldComponent = "component"
)

func withLogFields(ctx context.Context, f LogFields) context.Context {
	prev, _ := ctx.Value(logFieldsKey{}).(LogFields)
	merged := LogFields{}
	for k, v := range prev {
		merged[k] = v
	}
	for k, v := range f {
		merged[k] = v
	}
	return context.WithValue(ctx, logFieldsKey{}, merged)
}

func logFieldsFrom(ctx context.Context) []any {
	f, _ := ctx.Value(logFieldsKey{}).(LogFields)
	if len(f) == 0 {
		return nil
	}
	out := make([]any, 0, len(f)*2)
	for k, v := range f {
		out = append(out, k, v)
	}
	return out
}

func agentLog(ctx context.Context, msg string, args ...any) {
	all := append(logFieldsFrom(ctx), args...)
	slog.InfoContext(ctx, msg, all...)
}
