// Package audit 记录 Agent 会话 JSONL 审计、导出与敏感信息脱敏。
package audit

// SchemaVersion 是当前审计 JSONL  schema 版本号。
const SchemaVersion = 1

// Event is a single JSONL audit record (LLM-friendly, English snake_case keys).
type Event struct {
	V             int            `json:"v"`
	Ts            string         `json:"ts"`
	Event         string         `json:"event"`
	SessionID     string         `json:"session_id"`
	AgentID       string         `json:"agent_id,omitempty"`
	ParentAgentID string         `json:"parent_agent_id,omitempty"`
	ToolUseID     string         `json:"tool_use_id,omitempty"`
	Turn          int            `json:"turn,omitempty"`
	Component     string         `json:"component,omitempty"`
	Level         string         `json:"level,omitempty"`
	Data          map[string]any `json:"data,omitempty"`
}

// SessionMeta is persisted as {sessionID}.meta.json and updated on session end.
type SessionMeta struct {
	SessionID   string `json:"session_id"`
	StartedAt   string `json:"started_at"`
	EndedAt     string `json:"ended_at,omitempty"`
	Workspace   string `json:"workspace,omitempty"`
	Model       string `json:"model,omitempty"`
	TaskPreview string `json:"task_preview,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
	TurnCount   int    `json:"turn_count,omitempty"`
	DurationMs  int64  `json:"duration_ms,omitempty"`
	Error       string `json:"error,omitempty"`
}

// SessionIndex is a list entry for Bridge / ListSessions.
type SessionIndex struct {
	SessionID  string `json:"session_id"`
	StartedAt  string `json:"started_at"`
	EndedAt    string `json:"ended_at,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
	TurnCount  int    `json:"turn_count"`
	Path       string `json:"path"`
}

// ExportOptions controls ReadSession behavior.
type ExportOptions struct {
	MaxEvents int // 0 = all events
}

// ExportBundle is returned by ReadSession for Bridge export.
type ExportBundle struct {
	Meta      SessionMeta `json:"meta"`
	Events    []Event     `json:"events"`
	JSONLPath string      `json:"jsonl_path"`
	MetaPath  string      `json:"meta_path"`
	Subagents string      `json:"subagents_dir,omitempty"`
}

// DiagnosticDTO is the Wails-facing export payload.
type DiagnosticDTO struct {
	SessionID   string      `json:"session_id"`
	Meta        SessionMeta `json:"meta"`
	EventsTail  []Event     `json:"events_tail"`
	LLMMarkdown string      `json:"llm_markdown"`
	JSONLPath   string      `json:"jsonl_path"`
}
