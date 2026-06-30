// Package stream 定义 Agent 会话对外推送的流式消息（progress / stream_event / assistant / result）。
package stream

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	// TypeProgress 是 progress 类型顶层消息。
	TypeProgress = "progress"
	// TypeStreamEvent 是 stream_event 类型顶层消息。
	TypeStreamEvent = "stream_event"
	// TypeAssistant 是 assistant 类型顶层消息。
	TypeAssistant = "assistant"
	// TypeResult 是 result 类型顶层消息。
	TypeResult = "result"
	// TypeRunTerminal 是 Run 执行结束（SSE run:terminal）的顶层消息。
	TypeRunTerminal = "run_terminal"
	// TypeSubAgentUpdate 是子 Agent 进度快照更新。
	TypeSubAgentUpdate = "subagent_update"
	// TypeSubAgentDone 是子 Agent 结束快照。
	TypeSubAgentDone = "subagent_done"
)

const (
	// DataTurnProgress 是 TAOR 轮次进度 data.type。
	DataTurnProgress = "turn_progress"
	// DataMCPProgress 是 MCP 工具进度 data.type。
	DataMCPProgress = "mcp_progress"
	// DataToolProgress 是通用工具进度 data.type。
	DataToolProgress = "tool_progress"
	// DataToolOutputDelta 是工具执行中流式输出增量 data.type。
	DataToolOutputDelta = "tool_output_delta"
)

const (
	// EventMessageStart 是 message_start 流事件类型。
	EventMessageStart = "message_start"
	// EventContentBlockDelta 是 content_block_delta 流事件类型。
	EventContentBlockDelta = "content_block_delta"
	// EventMessageDelta 是 message_delta 流事件类型。
	EventMessageDelta = "message_delta"
	// EventMessageStop 是 message_stop 流事件类型。
	EventMessageStop = "message_stop"
)

const (
	// DeltaText 是 text_delta 内容块增量类型。
	DeltaText = "text_delta"
	// DeltaThinking 是 thinking_delta 内容块增量类型。
	DeltaThinking = "thinking_delta"
)

const (
	// ResultSuccess 是成功结束的 result.subtype。
	ResultSuccess = "success"
	// ResultErrorMaxTurns 是达到轮次上限的 result.subtype。
	ResultErrorMaxTurns = "error_max_turns"
	// ResultError 是错误结束的 result.subtype。
	ResultError = "error"
)

// Scope 区分 Coordinator 主会话与子 Worker 流式消息。
type Scope string

const (
	// ScopeCoordinator 表示 Coordinator 主会话流。
	ScopeCoordinator Scope = "coordinator"
	// ScopeWorker 表示子 Worker 流。
	ScopeWorker Scope = "worker"
)

// Message 为 Wails agent:stream 事件的 JSON 载荷。
type Message struct {
	Type            string  `json:"type"`
	SessionID       string  `json:"session_id,omitempty"`
	UUID            string  `json:"uuid,omitempty"`
	Scope           Scope   `json:"scope,omitempty"`
	AgentID         string  `json:"agent_id,omitempty"`
	ParentAgentID   string  `json:"parent_agent_id,omitempty"`
	ParentToolUseID *string `json:"parent_tool_use_id,omitempty"`

	// progress 类型字段
	ToolUseID string            `json:"tool_use_id,omitempty"`
	Data      *ToolProgressData `json:"data,omitempty"`

	// stream_event 类型字段
	Event *StreamEventPayload `json:"event,omitempty"`

	// assistant 类型字段
	Assistant *AssistantPayload `json:"message,omitempty"`

	// run_terminal 类型字段
	Status string `json:"status,omitempty"`

	// result 类型字段
	Subtype      string `json:"subtype,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	NumTurns     int    `json:"num_turns,omitempty"`
	DurationMs   int64  `json:"duration_ms,omitempty"`
	IsError      bool   `json:"is_error,omitempty"`
	ErrorMessage string `json:"error,omitempty"`
	Output       string `json:"output,omitempty"`

	// subagent 类型字段（JSON 与 agent.Snapshot 对齐）
	Snapshot any `json:"snapshot,omitempty"`
}

// ToolProgressData 为 progress 消息的 data 字段。
type ToolProgressData struct {
	Type          string `json:"type"`
	Status        string `json:"status,omitempty"`
	Turn          int    `json:"turn,omitempty"`
	Transition    string `json:"transition,omitempty"`
	Summary       string `json:"summary,omitempty"`
	ToolName      string `json:"tool_name,omitempty"`
	ServerName    string `json:"server_name,omitempty"`
	ElapsedTimeMs int64  `json:"elapsed_time_ms,omitempty"`
	Message       string `json:"message,omitempty"`
	Delta         string `json:"delta,omitempty"`
	OutputOffset  int64  `json:"output_offset,omitempty"`
}

// StreamEventPayload 为 stream_event.event。
type StreamEventPayload struct {
	Type  string      `json:"type"`
	Index int         `json:"index,omitempty"`
	Delta *BlockDelta `json:"delta,omitempty"`
	Usage any         `json:"usage,omitempty"`
}

// BlockDelta 为 content_block_delta 的 delta。
type BlockDelta struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
}

// AssistantPayload 为 assistant 消息体（简化 OpenAI 形态）。
type AssistantPayload struct {
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	ToolCalls  []ToolUseBlock `json:"tool_calls,omitempty"`
	StopReason string         `json:"stop_reason,omitempty"`
}

// ContentBlock 为 assistant content 块。
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
}

// ToolUseBlock 为 tool_use 块。
type ToolUseBlock struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"`
}

// newUUID 生成新的 UUID 字符串。
func newUUID() string {
	return uuid.NewString()
}

// base 构造带公共字段的基础流式消息。
func base(sessionID string) Message {
	return Message{
		SessionID: sessionID,
		UUID:      newUUID(),
		Scope:     ScopeCoordinator,
	}
}

// WithAgent 为消息打上子 Agent 归属标签（供 Worker 流式推送）。
func WithAgent(m Message, agentID, parentAgentID, parentToolUseID string) Message {
	m.Scope = ScopeWorker
	m.AgentID = agentID
	m.ParentAgentID = parentAgentID
	if parentToolUseID != "" {
		m.ParentToolUseID = &parentToolUseID
	}
	return m
}

// TagSink 包装 Sink，为每条出站消息附加 Agent 元数据。
type TagSink struct {
	Inner           Sink
	AgentID         string
	ParentAgentID   string
	ParentToolUseID string
}

// Publish 为出站消息附加 Agent 元数据后转发至 Inner。
func (t TagSink) Publish(ctx context.Context, msg Message) error {
	if t.Inner == nil {
		return nil
	}
	return t.Inner.Publish(ctx, WithAgent(msg, t.AgentID, t.ParentAgentID, t.ParentToolUseID))
}

// TurnProgress 新一轮 TAOR 迭代开始。
func TurnProgress(sessionID string, turn int, transition, summary string) Message {
	m := base(sessionID)
	m.Type = TypeProgress
	m.Data = &ToolProgressData{
		Type:       DataTurnProgress,
		Turn:       turn,
		Transition: transition,
		Summary:    summary,
	}
	return m
}

// ToolStarted 工具开始执行。
func ToolStarted(sessionID, toolUseID, toolName string, input string) Message {
	m := base(sessionID)
	m.Type = TypeProgress
	m.ToolUseID = toolUseID
	m.Data = &ToolProgressData{
		Type:     DataToolProgress,
		Status:   "started",
		ToolName: toolName,
		Message:  truncate(input, 500),
	}
	return m
}

// MCPProgress MCP 工具进度。
func MCPProgress(sessionID, toolUseID, status, serverName, toolName string, elapsedMs int64) Message {
	m := base(sessionID)
	m.Type = TypeProgress
	m.ToolUseID = toolUseID
	m.Data = &ToolProgressData{
		Type:          DataMCPProgress,
		Status:        status,
		ServerName:    serverName,
		ToolName:      toolName,
		ElapsedTimeMs: elapsedMs,
	}
	return m
}

// ToolOutputDelta 工具执行中流式输出增量。
func ToolOutputDelta(sessionID, toolUseID, toolName, delta string, outputOffset int64) Message {
	m := base(sessionID)
	m.Type = TypeProgress
	m.ToolUseID = toolUseID
	m.Data = &ToolProgressData{
		Type:         DataToolOutputDelta,
		Status:       "streaming",
		ToolName:     toolName,
		Delta:        delta,
		OutputOffset: outputOffset,
	}
	return m
}

// SubAgentUpdate 推送子 Agent 进度快照。
func SubAgentUpdate(sessionID string, snapshot any) Message {
	m := base(sessionID)
	m.Type = TypeSubAgentUpdate
	m.Snapshot = snapshot
	return m
}

// SubAgentDone 推送子 Agent 结束快照。
func SubAgentDone(sessionID string, snapshot any) Message {
	m := base(sessionID)
	m.Type = TypeSubAgentDone
	m.Snapshot = snapshot
	return m
}

// ToolInputStreaming 模型生成 tool_calls 参数时的增量（input_json_delta 对齐）。
func ToolInputStreaming(sessionID, toolUseID, toolName, delta string) Message {
	m := base(sessionID)
	m.Type = TypeProgress
	m.ToolUseID = toolUseID
	m.Data = &ToolProgressData{
		Type:     DataToolProgress,
		Status:   "input_streaming",
		ToolName: toolName,
		Delta:    delta,
	}
	return m
}

// ToolFinished 工具执行结束。
func ToolFinished(sessionID, toolUseID, toolName, status string, elapsedMs int64, outputPreview string) Message {
	m := base(sessionID)
	m.Type = TypeProgress
	m.ToolUseID = toolUseID
	m.Data = &ToolProgressData{
		Type:          DataToolProgress,
		Status:        status,
		ToolName:      toolName,
		ElapsedTimeMs: elapsedMs,
		Message:       truncate(outputPreview, 500),
	}
	return m
}

// TextDelta 流式文本增量。
func TextDelta(sessionID, text string, blockIndex int) Message {
	m := base(sessionID)
	m.Type = TypeStreamEvent
	m.Event = &StreamEventPayload{
		Type:  EventContentBlockDelta,
		Index: blockIndex,
		Delta: &BlockDelta{Type: DeltaText, Text: text},
	}
	return m
}

// ThinkingDelta 流式思考增量。
func ThinkingDelta(sessionID, thinking string, blockIndex int) Message {
	m := base(sessionID)
	m.Type = TypeStreamEvent
	m.Event = &StreamEventPayload{
		Type:  EventContentBlockDelta,
		Index: blockIndex,
		Delta: &BlockDelta{Type: DeltaThinking, Thinking: thinking},
	}
	return m
}

// MessageStart 新一轮 assistant 流开始。
func MessageStart(sessionID string) Message {
	m := base(sessionID)
	m.Type = TypeStreamEvent
	m.Event = &StreamEventPayload{Type: EventMessageStart}
	return m
}

// MessageStop assistant 流结束。
func MessageStop(sessionID string) Message {
	m := base(sessionID)
	m.Type = TypeStreamEvent
	m.Event = &StreamEventPayload{Type: EventMessageStop}
	return m
}

// Assistant 完整 assistant 轮次。
func Assistant(sessionID, text, thinking string, toolCalls []ToolUseBlock, stopReason string) Message {
	blocks := make([]ContentBlock, 0, 2)
	if thinking != "" {
		blocks = append(blocks, ContentBlock{Type: "thinking", Thinking: thinking})
	}
	if text != "" {
		blocks = append(blocks, ContentBlock{Type: "text", Text: text})
	}
	m := base(sessionID)
	m.Type = TypeAssistant
	m.Assistant = &AssistantPayload{
		Role:       "assistant",
		Content:    blocks,
		ToolCalls:  toolCalls,
		StopReason: stopReason,
	}
	return m
}

// RunTerminalMsg 构建 Run 结束事件（经 SSE event: run:terminal 推送）。
func RunTerminalMsg(sessionID, status, output, errMsg string) Message {
	m := base(sessionID)
	m.Type = TypeRunTerminal
	m.Status = status
	m.Output = output
	m.ErrorMessage = errMsg
	return m
}

// ResultSuccessMsg 正常结束。
func ResultSuccessMsg(sessionID, output, stopReason string, numTurns int, duration time.Duration) Message {
	m := base(sessionID)
	m.Type = TypeResult
	m.Subtype = ResultSuccess
	m.StopReason = stopReason
	m.NumTurns = numTurns
	m.DurationMs = duration.Milliseconds()
	m.Output = output
	return m
}

// ResultMaxTurns 达到最大轮次。
func ResultMaxTurns(sessionID string, numTurns int, duration time.Duration) Message {
	m := base(sessionID)
	m.Type = TypeResult
	m.Subtype = ResultErrorMaxTurns
	m.StopReason = "max_turns"
	m.NumTurns = numTurns
	m.DurationMs = duration.Milliseconds()
	m.IsError = true
	m.ErrorMessage = "已达到最大轮次上限"
	return m
}

// ResultErrorMsg 错误结束。
func ResultErrorMsg(sessionID, stopReason, errMsg string, numTurns int, duration time.Duration) Message {
	m := base(sessionID)
	m.Type = TypeResult
	m.Subtype = ResultError
	m.StopReason = stopReason
	m.NumTurns = numTurns
	m.DurationMs = duration.Milliseconds()
	m.IsError = true
	m.ErrorMessage = errMsg
	return m
}

// truncate 按 rune 数截断字符串。
func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
