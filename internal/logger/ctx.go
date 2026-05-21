package logger

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

// Fields are correlation attributes attached to context-scoped logs.
type Fields struct {
	SessionID string
	AgentID   string
	Turn      int
	Component string
}

// With stores logging fields in ctx.
func With(ctx context.Context, f Fields) context.Context {
	return context.WithValue(ctx, ctxKey{}, f)
}

// From reads logging fields from ctx.
func From(ctx context.Context) (Fields, bool) {
	if ctx == nil {
		return Fields{}, false
	}
	f, ok := ctx.Value(ctxKey{}).(Fields)
	return f, ok
}

func mergeAttrs(ctx context.Context, attrs []any) []any {
	f, ok := From(ctx)
	if !ok {
		return attrs
	}
	out := make([]any, 0, len(attrs)+8)
	if f.SessionID != "" {
		out = append(out, "session_id", f.SessionID)
	}
	if f.AgentID != "" {
		out = append(out, "agent_id", f.AgentID)
	}
	if f.Turn > 0 {
		out = append(out, "turn", f.Turn)
	}
	if f.Component != "" {
		out = append(out, "component", f.Component)
	}
	return append(out, attrs...)
}

// Log writes at the given level with context correlation fields.
func Log(ctx context.Context, level slog.Level, msg string, attrs ...any) {
	slog.Log(ctx, level, msg, mergeAttrs(ctx, attrs)...)
}

// InfoCtx logs Info with correlation fields.
func InfoCtx(ctx context.Context, msg string, attrs ...any) {
	Log(ctx, slog.LevelInfo, msg, attrs...)
}

// DebugCtx logs Debug with correlation fields.
func DebugCtx(ctx context.Context, msg string, attrs ...any) {
	Log(ctx, slog.LevelDebug, msg, attrs...)
}

// WarnCtx logs Warn with correlation fields.
func WarnCtx(ctx context.Context, msg string, attrs ...any) {
	Log(ctx, slog.LevelWarn, msg, attrs...)
}

// ErrorCtx logs Error with correlation fields.
func ErrorCtx(ctx context.Context, msg string, attrs ...any) {
	Log(ctx, slog.LevelError, msg, attrs...)
}
