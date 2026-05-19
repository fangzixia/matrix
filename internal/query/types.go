// Package query 实现 TAOR（思考→行动→观察→汇报）Agent 循环。
//
// 架构对标 claude-code 的 query.ts / QueryEngine.ts：
//
//	Run()           ← 外层会话入口（QueryEngine.ts）
//	  └─ queryLoop  ← for{} 状态机（query.ts）
//	       ├─ think  T：调用 LLM，流式接收响应
//	       ├─ act    A：执行工具调用
//	       ├─ observe O：将工具结果打包为用户消息
//	       └─ report  R：Stop Hook 检查 + 输出最终答案
//
// 上下文治理（对标 claude-code microCompact）由 [Config.PrepareHistory] 与
// [matrix/internal/session] 包组合实现，query 本包不直接依赖 session 以避免循环引用。
package query

import (
	"matrix/internal/llm"
	"matrix/internal/tools"
)

// Role 表示消息的发言方。
type Role string

const (
	// RoleSystem 表示系统角色消息，用于设置全局指令。
	RoleSystem Role = "system"
	// RoleUser 表示用户消息。
	RoleUser Role = "user"
	// RoleAssistant 表示助手（模型）消息。
	RoleAssistant Role = "assistant"
	// RoleTool 表示工具执行结果消息。
	RoleTool Role = "tool"
)

// Message 是对话历史中的单轮消息。
// Content 存储纯文本内容；工具调用与结果信息存放在专用字段中。
type Message struct {
	Role    Role
	Content string
	// Thinking 为助手消息中的完整思考内容，对应 AssistantTurn.Thinking。
	// [compat:deepseek] DeepSeek Reasoner 要求在后续请求中将 reasoning_content 原样回传，
	// 否则 API 返回 400；此字段用于跨轮次保存该内容。
	// [compat:claude]   Claude Extended Thinking 同理，但目前通过 Anthropic 原生 API 处理时
	// 需要将 thinking block 回传，OpenAI compat 端点行为视代理实现而定。
	// 标准 OpenAI 端点无此要求，此字段为空字符串时不影响正常调用。
	Thinking string
	// ToolCalls 在 assistant 消息中设置，表示模型请求调用的工具列表。
	ToolCalls []llm.ToolCall
	// ToolCallID 在工具结果消息（Role=RoleTool）中设置，用于关联对应的调用请求。
	ToolCallID string
	// ToolName 在工具结果消息中设置，记录执行的工具名称。
	ToolName string
	// IsError 为 true 表示该工具结果为执行失败。
	IsError bool
}

// TransitionReason 描述上一轮 TAOR 迭代的跃迁原因。
type TransitionReason string

const (
	// TransitionNextTurn 表示正常完成一轮工具调用后进入下一轮。
	TransitionNextTurn TransitionReason = "next_turn"
	// TransitionStopHookBlocking 表示 Stop Hook 注入了阻塞错误，强制重新推理。
	TransitionStopHookBlocking TransitionReason = "stop_hook_blocking"
	// TransitionMaxOutputTokensRecovery 表示触发输出 token 上限，尝试恢复。
	TransitionMaxOutputTokensRecovery TransitionReason = "max_output_tokens_recovery"
)

// state 是循环迭代间传递的可变内部状态。
type state struct {
	messages  []Message
	turnCount int
	// transition 记录上一轮跃迁原因；首次迭代为 nil。
	transition *TransitionReason
	// recoveryAttempts 记录当前连续 max_output_tokens 恢复尝试次数。
	recoveryAttempts int
}

// ContextPolicy defines the query-loop owned context management policy.
//
// It intentionally lives in query so every caller of Run, including coordinator
// workers and resumed workers, goes through the same pre-request pipeline.
type ContextPolicy struct {
	// MicroCompactThreshold is the estimated message-token threshold that
	// triggers clearing older tool results. 0 disables micro compaction.
	MicroCompactThreshold int
	// KeepRecentToolResults keeps the newest N tool results intact when micro
	// compaction runs. Values below 1 are normalized to 1.
	KeepRecentToolResults int
	// ClearedPlaceholder replaces old tool result content.
	ClearedPlaceholder string
	// ContextLimitTokens is the model context window. When set, Run prevents
	// estimated input + MaxTokens + margin from exceeding this value.
	ContextLimitTokens int
	// ContextSafetyMarginTokens reserves extra room for estimation error and
	// provider-side message/tool overhead.
	ContextSafetyMarginTokens int
	// MaxAsyncResultRunes limits injected async worker result messages.
	MaxAsyncResultRunes int
}

// Config 包含一个 TAOR 会话所需的全部配置参数。
type Config struct {
	// LLM 为 OpenAI 兼容的对话客户端（必填）。
	LLM *llm.Client
	// Model 为传递给 API 的模型名称，如 "gpt-4o" 或 "deepseek-chat"。
	Model string
	// SystemPrompt 为前置到每次 LLM 请求的系统角色消息。
	SystemPrompt string
	// InitialMessages 用于初始化对话历史，常用于会话恢复场景。
	InitialMessages []Message
	// Registry 为可用工具的注册表；nil 表示本次会话不使用工具。
	Registry *tools.Registry
	// MaxTurns 限制 TAOR 迭代轮次上限；0 表示不限。
	MaxTurns int
	// MaxTokens 为每次 LLM 请求的 max_tokens 提示值；0 表示由服务端决定。
	MaxTokens int
	// CanUseTool 为可选的工具权限检查回调；nil 表示允许所有工具执行。
	CanUseTool tools.CanUseToolFn
	// StopHook 在每次 R 阶段被调用。
	// 返回非空字符串则将其作为阻塞错误注入历史并重新进入循环；
	// 返回空字符串则正常结束。
	StopHook func(history []Message) string
	// AsyncResults 为异步子 Agent 完成时注入的 user 消息通道。
	// queryLoop 在 end_turn 时若 HasPendingAsync() 为 true 则阻塞等待，
	// 其余时机非阻塞 drain，模仿 claude-code 的 notifyOnCompletion 机制。
	// nil 表示无异步子 Agent 结果通道（嵌入方未使用 [coordinator.AsyncSupport]）。
	AsyncResults <-chan Message
	// HasPendingAsync 返回当前是否还有未完成的异步子 Agent。
	// nil 或返回 false 时，queryLoop 不等待异步结果，直接正常结束。
	HasPendingAsync func() bool
	// ContextPolicy configures query-loop owned context management.
	ContextPolicy ContextPolicy
	// MaxToolResultRunes 限制单条 tool 消息写入历史的最大字符数（按 Unicode rune 计）；
	// 超出部分截断并追加省略标记。0 表示不限制。
	MaxToolResultRunes int
	// LogPrefix 非空时，会拼入「第 N 轮」类事件与结构化日志，便于区分父子 Agent 的 TAOR 循环。
	LogPrefix string
}

// StopReason 描述 TAOR 循环终止的原因。
type StopReason string

const (
	// StopCompleted 表示模型正常结束（end_turn），无更多工具调用。
	StopCompleted StopReason = "completed"
	// StopMaxTurns 表示已达到 Config.MaxTurns 上限。
	StopMaxTurns StopReason = "max_turns"
	// StopAborted 表示 context 被取消（如用户中断）。
	StopAborted StopReason = "aborted"
	// StopModelError 表示遭遇不可恢复的 LLM API 错误。
	StopModelError StopReason = "model_error"
	// StopHookPause 表示 Stop Hook 返回了硬停止信号。
	StopHookPause StopReason = "hook_pause"
)

// Result 是 TAOR 循环退出时返回的最终结果。
type Result struct {
	// StopReason 描述本次循环的终止原因。
	StopReason StopReason
	// TurnCount 为实际执行的迭代轮次数。
	TurnCount int
	// Answer 为最后一轮模型输出的文本；循环提前终止时可能为空字符串。
	Answer string
	// Err 为循环中遇到的错误；正常结束时为 nil。
	Err error
	// Messages 是本次会话的完整消息历史（含 assistant、tool 消息）。
	// 供 SendMessage 工具在续接子 Agent 时作为初始历史传入。
	Messages []Message
}

// EventKind 标识实时流式事件的类型。
type EventKind string

const (
	// EventThinkingDelta 表示模型扩展思考的增量 token。
	EventThinkingDelta EventKind = "thinking_delta"
	// EventTextDelta 表示模型输出的文本增量 token。
	EventTextDelta EventKind = "text_delta"
	// EventToolCall 表示模型请求调用某个工具。
	EventToolCall EventKind = "tool_call"
	// EventToolResult 表示某个工具已执行完成。
	EventToolResult EventKind = "tool_result"
	// EventTurnStart 表示新一轮 TAOR 迭代开始。
	EventTurnStart EventKind = "turn_start"
	// EventDone 表示整个循环已结束，携带最终 Result。
	EventDone EventKind = "done"
)

// Event 通过可选的 events channel 发送，用于实时展示 TAOR 进度。
type Event struct {
	// Kind 为本次事件的类型。
	Kind EventKind
	// Delta 携带增量文本，由 EventThinkingDelta、EventTextDelta 和 EventTurnStart 使用。
	Delta string
	// ToolName 为工具名称，由 EventToolCall 和 EventToolResult 使用。
	ToolName string
	// ToolCallID 为工具调用 ID，由 EventToolCall 和 EventToolResult 使用。
	ToolCallID string
	// ToolInput 为工具调用的原始 JSON 参数字符串，由 EventToolCall 使用。
	ToolInput string
	// ToolOutput 为工具执行的输出内容，由 EventToolResult 使用。
	ToolOutput string
	// IsError 为 true 表示工具执行失败，由 EventToolResult 使用。
	IsError bool
	// Result 仅在 EventDone 事件中设置，携带循环最终结果。
	Result *Result
}
