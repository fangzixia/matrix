package query

// AuditRecorder 为可选的会话诊断写入器；nil 时不记录。
// 宿主可实现 concrete Writer（如 Matrix internal/modules/run/audit）。
type AuditRecorder interface {
	Emit(event string, turn int, component string, data map[string]any)
	EmitWithTool(event string, turn int, component, toolUseID string, data map[string]any)
}
