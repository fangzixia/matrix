package logging

import (
	"context"
	"fmt"
	"log/slog"
)

// Fields 是写入 context 的结构化日志键值对。
type Fields map[string]string

const (
	// FieldSessionID 是会话 ID 日志字段名。
	FieldSessionID = "session_id"
	// FieldComponent 是组件名日志字段名。
	FieldComponent = "component"
	// FieldTurn 是对话轮次日志字段名。
	FieldTurn = "turn"
)

type ctxKey struct{}

// With 将 Fields 附加到 context，供 InfoCtx 等函数合并输出。
func With(ctx context.Context, f Fields) context.Context {
	return context.WithValue(ctx, ctxKey{}, f)
}

func fieldsFrom(ctx context.Context) []any {
	v := ctx.Value(ctxKey{})
	if v == nil {
		return nil
	}
	m, _ := v.(Fields)
	args := make([]any, 0, len(m)*2)
	for k, val := range m {
		args = append(args, k, val)
	}
	return args
}

// InfoCtx 写入 info 级别日志，并合并 context 中的 Fields。
func InfoCtx(ctx context.Context, msg string, args ...any) {
	all := append(fieldsFrom(ctx), args...)
	slog.Info(msg, all...)
}

// Infof 以格式化字符串写入 info 级别日志。
func Infof(format string, args ...any) { slog.Info(fmt.Sprintf(format, args...)) }

// Warnf 以格式化字符串写入 warn 级别日志。
func Warnf(format string, args ...any) { slog.Warn(fmt.Sprintf(format, args...)) }

// Errorf 以格式化字符串写入 error 级别日志。
func Errorf(format string, args ...any) { slog.Error(fmt.Sprintf(format, args...)) }
