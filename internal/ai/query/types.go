// Package query 实现 TAOR（思考→行动→观察→汇报）Agent 循环。
//
//	Run()           ← 外层会话入口
//	  └─ queryLoop  ← for{} 状态机
//	       ├─ think  T：调用 LLM，流式接收响应
//	       ├─ act    A：执行工具调用
//	       ├─ observe O：将工具结果打包为用户消息
//	       └─ report  R：Stop Hook 检查 + 输出最终答案
//
// 上下文治理（microCompact / autoCompact）见 context_pipeline.go。
package query

import (
	"matrix/internal/ai/audit"
	"matrix/internal/ai/llm"
	"matrix/internal/ai/stream"
	"matrix/internal/ai/tools"
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
)

// state 是循环迭代间传递的可变内部状态。
type state struct {
	messages  []Message
	turnCount int
	// transition 记录上一轮跃迁原因；首次迭代为 nil。
	transition *TransitionReason
}

// ContextPolicy 定义 query 循环统一的上下文管理策略。
//
// 放在 query 包中，使 Coordinator Worker 与恢复中的 Worker 均走同一套 pre-request 流水线。
type ContextPolicy struct {
	// MicroCompactThreshold 为触发清理较早工具结果的估算消息 token 阈值；0 表示禁用微压缩。
	MicroCompactThreshold int
	// KeepRecentToolResults 在微压缩时保留最近 N 条工具结果；小于 1 的值会规范化为 1。
	KeepRecentToolResults int
	// ClearedPlaceholder 用于替换被清理的旧工具结果内容。
	ClearedPlaceholder string
	// ContextLimitTokens 为模型上下文窗口大小；设置后 Run 会阻止估算输入 + MaxTokens + 余量超过该值。
	ContextLimitTokens int
	// ContextSafetyMarginTokens 为估算误差及提供方消息/工具开销预留额外 token 余量。
	ContextSafetyMarginTokens int
	// MaxAsyncResultRunes 限制注入的异步 Worker 结果消息长度（按 rune 计）。
	MaxAsyncResultRunes int
	// AutoCompactThreshold 为估算 token 达到该值时触发 LLM 全量会话摘要（0 禁用）。
	// 达到该阈值时触发 LLM 全量会话摘要。
	AutoCompactThreshold int
	// KeepRecentMessages 为 LLM 全量摘要后保留的最近消息条数（含 user/assistant/tool）。
	KeepRecentMessages int
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
	// 其余时机非阻塞 drain 异步子 Agent 结果。
	// nil 表示无异步子 Agent 结果通道（嵌入方未使用 [coordinator.AsyncSupport]）。
	AsyncResults <-chan Message
	// HasPendingAsync 返回当前是否还有未完成的异步子 Agent。
	// nil 或返回 false 时，queryLoop 不等待异步结果，直接正常结束。
	HasPendingAsync func() bool
	// ContextPolicy 配置 query 循环负责的上下文管理策略。
	ContextPolicy ContextPolicy
	// MaxToolResultRunes 限制单条 tool 消息写入历史的最大字符数（按 Unicode rune 计）；
	// 超出部分截断并追加省略标记。0 表示不限制。
	MaxToolResultRunes int
	// LogPrefix 非空时，会拼入「第 N 轮」类事件与结构化日志，便于区分父子 Agent 的 TAOR 循环。
	LogPrefix string
	// SessionID 为流式 SDK 消息的会话标识；空则由 RunSession 生成。
	SessionID string
	// Audit 为会话诊断 JSONL 写入器；nil 时不记录诊断事件。
	Audit *audit.Writer
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

// StreamSink 为 RunSession 的过程消息出口（见 matrix/internal/ai/stream）。
type StreamSink = stream.Sink
