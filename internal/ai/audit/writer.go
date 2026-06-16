package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Writer appends session diagnostic events to sessionsDir.
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

// NewWriter creates audit JSONL under sessionsDir.
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

// SetAgentContext sets optional agent lineage for subsequent events.
func (w *Writer) SetAgentContext(agentID, parentAgentID string) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.agentID = agentID
	w.parentID = parentAgentID
}

// SessionID returns the bound session id.
func (w *Writer) SessionID() string {
	if w == nil {
		return ""
	}
	return w.sessionID
}

// JSONLPath returns the events file path.
func (w *Writer) JSONLPath() string {
	if w == nil {
		return ""
	}
	return w.jsonlPath
}

// Emit appends one audit event; failures are silent.
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

// EmitWithTool is Emit with tool_use_id set.
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

// UpdateMeta merges fields into meta.json (e.g. model, task on session.start).
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

// Close finalizes meta.json with session end fields.
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

func (w *Writer) writeMeta(meta SessionMeta) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writeMetaLocked(meta)
}

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
