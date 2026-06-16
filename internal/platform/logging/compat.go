package logging

import (
	"context"
	"fmt"
	"log/slog"
)

type Fields map[string]string

const (
	FieldSessionID = "session_id"
	FieldComponent = "component"
	FieldTurn      = "turn"
)

type ctxKey struct{}

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

func InfoCtx(ctx context.Context, msg string, args ...any) {
	all := append(fieldsFrom(ctx), args...)
	slog.Info(msg, all...)
}

func Infof(format string, args ...any)  { slog.Info(fmt.Sprintf(format, args...)) }
func Warnf(format string, args ...any)  { slog.Warn(fmt.Sprintf(format, args...)) }
func Errorf(format string, args ...any) { slog.Error(fmt.Sprintf(format, args...)) }
