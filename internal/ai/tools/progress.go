package tools

import (
	"context"
	"matrix/internal/ai/stream"
	"sync"
)

type reporterKey struct{}

// OutputReporter 向 SSE 推送工具执行中的流式输出。
type OutputReporter struct {
	SessionID string
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

// EmitOutput 推送工具输出增量（并写入 spill 文件）。
func EmitOutput(ctx context.Context, chunk string) {
	if chunk == "" {
		return
	}
	rep := ReporterFromContext(ctx)
	if rep == nil || rep.Emit == nil {
		return
	}
	if rep.Spill != nil {
		_ = rep.Spill.Append(chunk)
	}
	rep.mu.Lock()
	rep.offset += int64(len(chunk))
	off := rep.offset
	rep.mu.Unlock()
	rep.Emit(rep.ToolUseID, stream.ToolProgressData{
		Type:         stream.DataToolOutputDelta,
		Status:       "streaming",
		ToolName:     rep.ToolName,
		Delta:        chunk,
		OutputOffset: off,
	})
}

// EmitStatus 推送非输出类进度行（如「扫描中…」）。
func EmitStatus(ctx context.Context, line string) {
	if line == "" {
		return
	}
	EmitOutput(ctx, line+"\n")
}

// SpillPathFromContext 返回当前工具 spill 路径（若有）。
func SpillPathFromContext(ctx context.Context) string {
	rep := ReporterFromContext(ctx)
	if rep == nil || rep.Spill == nil {
		return ""
	}
	return rep.Spill.Path()
}
