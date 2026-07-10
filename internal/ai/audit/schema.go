// Package audit 记录 Agent 会话 JSONL 审计、导出与敏感信息脱敏。
package audit

// SchemaVersion 是当前审计 JSONL  schema 版本号。
const SchemaVersion = 1

// Event 是单条 JSONL 审计记录（面向 LLM，字段为英文 snake_case）。
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

// SessionMeta 持久化为 {sessionID}.meta.json，会话结束时更新。
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
