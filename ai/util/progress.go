package util

import (
	"context"
	"io"
	"matrix/ai/stream"
	"sync"
)

type reporterKey struct{}
type streamWriterKey struct{}

// OutputReporter 向 Sink 推送工具执行中的流式输出。
type OutputReporter struct {
	ToolUseID string
	ToolName  string
	Emit      ProgressFn
	Spill     *OutputSpillWriter
	mu        sync.Mutex
	offset    int64
}

// ContextWithReporter 将 OutputReporter 注入 context。
func ContextWithReporter(ctx context.Context, rep *OutputReporter) context.Context {
	if rep == nil {
		return ctx
	}
	return context.WithValue(ctx, reporterKey{}, rep)
}

// ReporterFromContext 读取 OutputReporter。
func ReporterFromContext(ctx context.Context) *OutputReporter {
	if v, ok := ctx.Value(reporterKey{}).(*OutputReporter); ok {
		return v
	}
	return nil
}

// WithStreamWriter 将 io.Writer 注入 context（由 execOne 设置）。
func WithStreamWriter(ctx context.Context, w io.Writer) context.Context {
	if w == nil {
		return ctx
	}
	return context.WithValue(ctx, streamWriterKey{}, w)
}

// StreamWriter 返回工具执行期可写的流；无 reporter 时为 io.Discard。
func StreamWriter(ctx context.Context) io.Writer {
	if w, ok := ctx.Value(streamWriterKey{}).(io.Writer); ok && w != nil {
		return w
	}
	return io.Discard
}

type reporterWriter struct {
	ctx context.Context
	rep *OutputReporter
}

func newReporterWriter(ctx context.Context, rep *OutputReporter) io.Writer {
	if rep == nil || rep.Emit == nil {
		return io.Discard
	}
	return reporterWriter{ctx: ctx, rep: rep}
}

func (w reporterWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	emitOutput(w.ctx, w.rep, string(p))
	return len(p), nil
}

func emitOutput(ctx context.Context, rep *OutputReporter, chunk string) {
	if chunk == "" || rep == nil || rep.Emit == nil {
		return
	}
	if rep.Spill != nil {
		_ = rep.Spill.Append(chunk)
	}
	rep.mu.Lock()
	rep.offset += int64(len(chunk))
	rep.mu.Unlock()
	rep.Emit(stream.ToolOutputDelta(rep.ToolUseID, rep.ToolName, chunk))
}

// WriteString 向 StreamWriter 写入字符串。
func WriteString(ctx context.Context, s string) {
	if s == "" {
		return
	}
	_, _ = io.WriteString(StreamWriter(ctx), s)
}

// SpillPathFromContext 返回当前工具 spill 路径（若有）。
func SpillPathFromContext(ctx context.Context) string {
	rep := ReporterFromContext(ctx)
	if rep == nil || rep.Spill == nil {
		return ""
	}
	return rep.Spill.Path()
}
