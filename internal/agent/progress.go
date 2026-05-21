package agent

// Progress 描述子 Agent 运行中的可观测状态（对齐 Claude Code task progress 子集）。
type Progress struct {
	Turn         int    `json:"turn"`
	Transition   string `json:"transition,omitempty"`
	Summary      string `json:"summary,omitempty"`
	CurrentTool  string `json:"current_tool,omitempty"`
	ToolUseCount int    `json:"tool_use_count"`
	LastActivity string `json:"last_activity,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}

// Snapshot 为前端 / Wails 暴露的子 Agent 只读视图。
type Snapshot struct {
	ID              string   `json:"id"`
	Description     string   `json:"description"`
	Status          Status   `json:"status"`
	ParentAgentID   string   `json:"parent_agent_id,omitempty"`
	ParentToolUseID string   `json:"parent_tool_use_id,omitempty"`
	Progress        Progress `json:"progress"`
	CreatedAt       int64    `json:"created_at"`
	SidechainPath   string   `json:"sidechain_path,omitempty"`
	AnswerPreview   string   `json:"answer_preview,omitempty"`
	TurnCount       int      `json:"turn_count,omitempty"`
}
