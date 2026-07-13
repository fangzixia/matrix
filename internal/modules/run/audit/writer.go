package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Writer 将会话诊断事件追加写入 sessionsDir。
type Writer struct {
	mu        sync.Mutex
	root      string
	sessionID string
	jsonlPath string
	metaPath  string
	started   time.Time
	agentID   string
	parentID  string
}

// NewWriter 在 sessionsDir 下创建审计 JSONL。
func NewWriter(sessionsDir, workspacePath, sessionID string) *Writer {
	if sessionsDir == "" || sessionID == "" {
		return &Writer{}
	}
	dir := sessionsDir
	_ = os.MkdirAll(dir, 0o755)
	now := time.Now().UTC()
	w := &Writer{
		root:      workspacePath,
		sessionID: sessionID,
		jsonlPath: filepath.Join(dir, sessionID+".jsonl"),
		metaPath:  filepath.Join(dir, sessionID+".meta.json"),
		started:   now,
	}
	w.writeMeta(SessionMeta{
		SessionID: sessionID,
		StartedAt: formatTimeUTC(now),
		Workspace: workspacePath,
	})
	return w
}

// SetAgentContext 为后续事件设置可选的 Agent 血缘信息。
func (w *Writer) SetAgentContext(agentID, parentAgentID string) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.agentID = agentID
	w.parentID = parentAgentID
}

// SessionID 返回绑定的会话 ID。
func (w *Writer) SessionID() string {
	if w == nil {
		return ""
	}
	return w.sessionID
}

// JSONLPath 返回事件文件路径。
func (w *Writer) JSONLPath() string {
	if w == nil {
		return ""
	}
	return w.jsonlPath
}

// Emit 追加一条审计事件；失败时静默忽略。
func (w *Writer) Emit(event string, turn int, component string, data map[string]any) {
	if w == nil || w.sessionID == "" {
		return
	}
	ev := Event{
		V:             SchemaVersion,
		Ts:            time.Now().UTC().Format(time.RFC3339Nano),
		Event:         event,
		SessionID:     w.sessionID,
		AgentID:       w.agentID,
		ParentAgentID: w.parentID,
		Turn:          turn,
		Component:     component,
		Level:         "info",
		Data:          PreviewKeys(data),
	}
	w.appendEvent(ev)
}

// EmitWithTool 等同于 Emit，并设置 tool_use_id。
func (w *Writer) EmitWithTool(event string, turn int, component, toolUseID string, data map[string]any) {
	if w == nil || w.sessionID == "" {
		return
	}
	ev := Event{
		V:             SchemaVersion,
		Ts:            time.Now().UTC().Format(time.RFC3339Nano),
		Event:         event,
		SessionID:     w.sessionID,
		AgentID:       w.agentID,
		ParentAgentID: w.parentID,
		ToolUseID:     toolUseID,
		Turn:          turn,
		Component:     component,
		Level:         "info",
		Data:          PreviewKeys(data),
	}
	w.appendEvent(ev)
}

// UpdateMeta 将字段合并写入 meta.json（如 session.start 时的 model、task）。
func (w *Writer) UpdateMeta(patch SessionMeta) {
	if w == nil || w.metaPath == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	meta := w.readMetaLocked()
	if patch.Model != "" {
		meta.Model = patch.Model
	}
	if patch.TaskPreview != "" {
		meta.TaskPreview = patch.TaskPreview
	}
	if patch.Workspace != "" {
		meta.Workspace = patch.Workspace
	}
	if patch.StartedAt != "" {
		meta.StartedAt = patch.StartedAt
	}
	w.writeMetaLocked(meta)
}

// Close 写入会话结束字段并完成 meta.json。
func (w *Writer) Close(meta SessionMeta) error {
	if w == nil || w.metaPath == "" {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	m := w.readMetaLocked()
	if meta.StopReason != "" {
		m.StopReason = meta.StopReason
	}
	if meta.TurnCount > 0 {
		m.TurnCount = meta.TurnCount
	}
	if meta.DurationMs > 0 {
		m.DurationMs = meta.DurationMs
	}
	if meta.Error != "" {
		m.Error = RedactString(meta.Error)
	}
	if meta.EndedAt != "" {
		m.EndedAt = meta.EndedAt
	} else {
		m.EndedAt = formatTimeUTC(time.Now())
	}
	w.writeMetaLocked(m)
	return nil
}

// appendEvent 将审计事件追加写入 JSONL 文件。
func (w *Writer) appendEvent(ev Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	f, err := os.OpenFile(w.jsonlPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	_ = enc.Encode(ev)
}

// writeMeta 写入会话 meta.json（加锁）。
func (w *Writer) writeMeta(meta SessionMeta) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writeMetaLocked(meta)
}

// readMetaLocked 在持锁状态下读取 meta.json。
func (w *Writer) readMetaLocked() SessionMeta {
	meta := SessionMeta{SessionID: w.sessionID, StartedAt: formatTimeUTC(w.started)}
	data, err := os.ReadFile(w.metaPath)
	if err != nil {
		return meta
	}
	_ = json.Unmarshal(data, &meta)
	if meta.SessionID == "" {
		meta.SessionID = w.sessionID
	}
	return meta
}

// writeMetaLocked 在持锁状态下写入 meta.json。
func (w *Writer) writeMetaLocked(meta SessionMeta) {
	if meta.SessionID == "" {
		meta.SessionID = w.sessionID
	}
	if meta.StartedAt == "" {
		meta.StartedAt = formatTimeUTC(w.started)
	}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(w.metaPath, b, 0o644)
}
