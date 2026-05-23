package desktop

import (
	"fmt"

	"matrix/internal/audit"
	"matrix/internal/matrixpaths"
)

// ListSessionDiagnostics 列出工作区下最近的会话诊断记录。
func (b *Bridge) ListSessionDiagnostics(limit int) ([]audit.SessionIndex, error) {
	if limit <= 0 {
		limit = 20
	}
	return audit.ListSessions(b.workspaceRoot(), limit)
}

// GetSessionDiagnostic 读取指定会话的诊断包（含 LLM 可粘贴的 Markdown）。
func (b *Bridge) GetSessionDiagnostic(sessionID string) (*audit.DiagnosticDTO, error) {
	if sessionID == "" {
		b.sessions.mu.Lock()
		sessionID = b.sessions.sessionID
		b.sessions.mu.Unlock()
	}
	if sessionID == "" {
		return nil, fmt.Errorf("session_id 为空且无活动会话")
	}
	bundle, err := audit.ReadSession(b.workspaceRoot(), sessionID, audit.ExportOptions{MaxEvents: 0})
	if err != nil {
		return nil, err
	}
	dto := audit.ToDiagnosticDTO(bundle, 200)
	return &dto, nil
}

// ExportSessionDiagnosticToFile 将会话诊断导出为 Markdown（及 JSONL 副本）到 destDir。
func (b *Bridge) ExportSessionDiagnosticToFile(sessionID, destDir string) (mdPath string, jsonlPath string, err error) {
	if sessionID == "" {
		return "", "", fmt.Errorf("session_id 不能为空")
	}
	if destDir == "" {
		destDir = matrixpaths.ExportsDir(b.workspaceRoot())
	}
	bundle, err := audit.ReadSession(b.workspaceRoot(), sessionID, audit.ExportOptions{})
	if err != nil {
		return "", "", err
	}
	return audit.WriteExportFiles(bundle, destDir)
}
