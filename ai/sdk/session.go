package sdk

import "matrix/ai/query"

// Config 为单次 RunSession 的运行配置。
type Config = query.Config

// Message 为会话消息。
type Message = query.Message

// MessageAttachment 为消息附件。
type MessageAttachment = query.MessageAttachment

// Result 为 RunSession 的同步返回结果。
type Result = query.Result

// ContextPolicy 为上下文压缩与裁剪策略。
type ContextPolicy = query.ContextPolicy

// Role 为消息角色。
type Role = query.Role

// StopReason 为会话结束原因。
type StopReason = query.StopReason

// TransitionReason 为状态转换原因。
type TransitionReason = query.TransitionReason

// AuditRecorder 为可选的会话诊断写入接口。
type AuditRecorder = query.AuditRecorder

// StreamSink 为 query 层使用的流式输出接口。
type StreamSink = query.StreamSink

const (
	// RoleSystem 表示系统消息。
	RoleSystem = query.RoleSystem
	// RoleUser 表示用户消息。
	RoleUser = query.RoleUser
	// RoleAssistant 表示助手消息。
	RoleAssistant = query.RoleAssistant
	// RoleTool 表示工具结果消息。
	RoleTool = query.RoleTool
	// StopCompleted 表示正常完成。
	StopCompleted = query.StopCompleted
	// StopMaxTurns 表示达到最大轮次。
	StopMaxTurns = query.StopMaxTurns
	// StopAborted 表示被取消或中止。
	StopAborted = query.StopAborted
	// StopModelError 表示模型调用失败。
	StopModelError = query.StopModelError
)

var (
	// RunSession 启动一次 Agent 会话（TAOR 循环）。
	RunSession = query.RunSession
	// PreviewText 生成文本预览（截断）。
	PreviewText = query.PreviewText
	// TruncateRunes 按 rune 截断字符串。
	TruncateRunes = query.TruncateRunes
)
