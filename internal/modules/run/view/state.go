package view

// RunViewState 是 Run 活动视图的权威快照。
type RunViewState struct {
	RunID          string                  `json:"runId"`
	Seq            int64                   `json:"seq"`
	Status         string                  `json:"status"`
	Phase          string                  `json:"phase,omitempty"`
	StatusLabel    string                  `json:"statusLabel"`
	ReplyText      string                  `json:"replyText"`
	ReplyMessageID string                  `json:"replyMessageId,omitempty"`
	Turns          []TurnView              `json:"turns"`
	Subagents      map[string]SubagentView `json:"subagents,omitempty"`
	Result         *ResultView             `json:"result,omitempty"`
	Error          string                  `json:"error,omitempty"`
}

// TurnView 表示一轮 TAOR 活动。
type TurnView struct {
	Key               string     `json:"key"`
	Turn              int        `json:"turn"`
	Scope             string     `json:"scope"`
	AgentID           string     `json:"agentId,omitempty"`
	ParentToolUseID   string     `json:"parentToolUseId,omitempty"`
	Summary           string     `json:"summary,omitempty"`
	Thinking          string     `json:"thinking"`
	ThinkingStreaming bool       `json:"thinkingStreaming"`
	Message           string     `json:"message"`
	MessageStreaming  bool       `json:"messageStreaming"`
	Tools             []ToolView `json:"tools"`
}

// ToolView 表示工具执行步骤。
type ToolView struct {
	ToolCallID      string     `json:"toolCallId"`
	ToolCallName    string     `json:"toolCallName"`
	Status          string     `json:"status"`
	Preview         string     `json:"preview,omitempty"`
	LiveOutput      string     `json:"liveOutput,omitempty"`
	OutputStreaming bool       `json:"outputStreaming"`
	ElapsedMs       int64      `json:"elapsedMs,omitempty"`
	ServerName      string     `json:"serverName,omitempty"`
	LogURL          string     `json:"logUrl,omitempty"`
	WorkerTurns     []TurnView `json:"workerTurns,omitempty"`
}

// SubagentView 子 Agent 快照。
type SubagentView struct {
	ID              string         `json:"id"`
	Description     string         `json:"description,omitempty"`
	Status          string         `json:"status,omitempty"`
	ParentAgentID   string         `json:"parent_agent_id,omitempty"`
	ParentToolUseID string         `json:"parent_tool_use_id,omitempty"`
	Progress        map[string]any `json:"progress,omitempty"`
	CreatedAt       int64          `json:"created_at,omitempty"`
	SidechainPath   string         `json:"sidechain_path,omitempty"`
	AnswerPreview   string         `json:"answer_preview,omitempty"`
	TurnCount       int            `json:"turn_count,omitempty"`
}

// ResultView 会话结束结果。
type ResultView struct {
	Subtype    string `json:"subtype,omitempty"`
	Output     string `json:"output,omitempty"`
	IsError    bool   `json:"isError,omitempty"`
	Error      string `json:"error,omitempty"`
	NumTurns   int    `json:"numTurns,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
	StopReason string `json:"stopReason,omitempty"`
}

// NewRunViewState 创建初始 Run 视图状态。
func NewRunViewState(runID string) RunViewState {
	return RunViewState{
		RunID:       runID,
		Status:      "running",
		StatusLabel: "Agent 正在工作…",
		Turns:       []TurnView{},
	}
}
