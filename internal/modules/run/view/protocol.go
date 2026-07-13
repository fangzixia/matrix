// Package view 将内部 stream.Message 投影为 AG-UI 对齐的 RunViewState；
// 持久化到 run_views，SSE 只读 DB 轮询推送。
package view

// AG-UI 对齐的事件类型。
//
// 当前对外 SSE 以 DB 快照流为权威：CatchUpAfterSeq 只稳定回放
// STATE_SNAPSHOT/RUN_STARTED/ACTIVITY_SNAPSHOT/RUN_FINISHED。细粒度
// TEXT/TOOL/STEP 事件是投影器内部语义，只有引入持久化事件日志后才可作为
// 可恢复的外部事件流契约。
const (
	EventRUNStarted              = "RUN_STARTED"
	EventRUNFinished             = "RUN_FINISHED"
	EventRUNError                = "RUN_ERROR"
	EventTEXTMessageStart        = "TEXT_MESSAGE_START"
	EventTEXTMessageContent      = "TEXT_MESSAGE_CONTENT"
	EventTEXTMessageEnd          = "TEXT_MESSAGE_END"
	EventREASONINGMessageContent = "REASONING_MESSAGE_CONTENT"
	EventREASONINGMessageEnd     = "REASONING_MESSAGE_END"
	EventTOOLCallStart           = "TOOL_CALL_START"
	EventTOOLCallArgs            = "TOOL_CALL_ARGS"
	EventTOOLCallEnd             = "TOOL_CALL_END"
	EventTOOLCallResult          = "TOOL_CALL_RESULT"
	EventACTIVITYSnapshot        = "ACTIVITY_SNAPSHOT"
	EventSTATESnapshot           = "STATE_SNAPSHOT"
)

// Mode 是 SSE 订阅通道模式。
type Mode string

const (
	ModeChat   Mode = "chat"
	ModeDetail Mode = "detail"
)

// Envelope 是对外 SSE/REST 事件载荷。
type Envelope struct {
	Type      string `json:"type"`
	RunID     string `json:"runId"`
	Seq       int64  `json:"seq"`
	Timestamp int64  `json:"timestamp"`
	Payload   any    `json:"payload"`
}

// RUN_STARTED payload.
type RunStartedPayload struct {
	StatusLabel string `json:"statusLabel"`
	Phase       string `json:"phase,omitempty"`
	Kind        string `json:"kind,omitempty"`
}

// RUN_FINISHED payload.
type RunFinishedPayload struct {
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// RUN_ERROR payload.
type RunErrorPayload struct {
	Message string `json:"message"`
}

// TextMessageStartPayload 文本流开始。
type TextMessageStartPayload struct {
	MessageID string `json:"messageId"`
	Scope     string `json:"scope,omitempty"`
	AgentID   string `json:"agentId,omitempty"`
}

// TextDeltaPayload 文本/思考增量。
type TextDeltaPayload struct {
	MessageID string `json:"messageId"`
	Delta     string `json:"delta"`
}

// TextMessageEndPayload 文本流结束。
type TextMessageEndPayload struct {
	MessageID string `json:"messageId"`
}

// ToolCallStartPayload 工具调用开始。
type ToolCallStartPayload struct {
	ToolCallID   string `json:"toolCallId"`
	ToolCallName string `json:"toolCallName"`
	ServerName   string `json:"serverName,omitempty"`
}

// ToolCallArgsPayload 工具参数流。
type ToolCallArgsPayload struct {
	ToolCallID string `json:"toolCallId"`
	Delta      string `json:"delta"`
}

// ToolCallEndPayload 工具参数结束。
type ToolCallEndPayload struct {
	ToolCallID string `json:"toolCallId"`
}

// ToolCallResultPayload 工具执行结果。
type ToolCallResultPayload struct {
	ToolCallID string `json:"toolCallId"`
	Status     string `json:"status"`
	Preview    string `json:"preview,omitempty"`
	ElapsedMs  int64  `json:"elapsedMs,omitempty"`
	LogURL     string `json:"logUrl,omitempty"`
}

// ActivitySnapshotPayload 活动快照（子 Agent 等）。
type ActivitySnapshotPayload struct {
	Subagents   map[string]SubagentView `json:"subagents,omitempty"`
	StatusLabel string                  `json:"statusLabel,omitempty"`
}

// AllowedInChat 判断事件是否允许 chat 通道。
// chat 除回复文本外，还需子 Agent / 工具活动（ACTIVITY_SNAPSHOT + STATE_SNAPSHOT）。
func AllowedInChat(eventType string) bool {
	switch eventType {
	case EventRUNStarted, EventRUNFinished, EventRUNError, EventTEXTMessageContent,
		EventACTIVITYSnapshot, EventSTATESnapshot, "job_run_finished":
		return true
	default:
		return false
	}
}
