package sdk

import (
	"context"
	"time"

	"matrix/ai/stream"
)

// Event 为 AG-UI 事件。
type Event = stream.Event

// Sink 为 AG-UI 事件发布接口。
type Sink = stream.Sink

// NopSink 为丢弃所有事件的 Sink。
type NopSink = stream.NopSink

// FuncSink 将 Publish 委托给函数的 Sink。
type FuncSink = stream.FuncSink

// ChanSink 将事件写入 channel 的 Sink。
type ChanSink = stream.ChanSink

// CoalesceSink 合并相邻文本增量的 Sink。
type CoalesceSink = stream.CoalesceSink

// OutputCoalesceSink 合并工具输出增量的 Sink。
type OutputCoalesceSink = stream.OutputCoalesceSink

// ScopeSink 为带作用域元数据的 Sink 包装。
type ScopeSink = stream.ScopeSink

// Meta 为事件元数据。
type Meta = stream.Meta

const (
	// ActivityTypeSubagent 为子 Agent 活动类型常量。
	ActivityTypeSubagent = stream.ActivityTypeSubagent
	// CustomNameToolOutputDelta 为工具输出增量自定义事件名。
	CustomNameToolOutputDelta = stream.CustomNameToolOutputDelta
)

var (
	// NewRunID 生成新的 runId。
	NewRunID = stream.NewRunID
	// NewMessageID 生成新的 messageId。
	NewMessageID = stream.NewMessageID
	// EventType 返回事件类型字符串。
	EventType = stream.EventType
	// WithMeta 将 Meta 写入 context。
	WithMeta = stream.WithMeta
	// MetaFrom 从 context 读取 Meta。
	MetaFrom = stream.MetaFrom
	// SubagentActivity 构造子 Agent 活动事件。
	SubagentActivity = stream.SubagentActivity
	// RunStarted 构造 RUN_STARTED 事件。
	RunStarted = stream.RunStarted
	// RunFinished 构造 RUN_FINISHED 事件。
	RunFinished = stream.RunFinished
	// RunError 构造 RUN_ERROR 事件。
	RunError = stream.RunError
	// StepStarted 构造 STEP_STARTED 事件。
	StepStarted = stream.StepStarted
	// StepFinished 构造 STEP_FINISHED 事件。
	StepFinished = stream.StepFinished
	// TextMessageStart 构造文本消息开始事件。
	TextMessageStart = stream.TextMessageStart
	// TextMessageContent 构造文本消息内容增量事件。
	TextMessageContent = stream.TextMessageContent
	// TextMessageEnd 构造文本消息结束事件。
	TextMessageEnd = stream.TextMessageEnd
	// ReasoningMessageStart 构造推理消息开始事件。
	ReasoningMessageStart = stream.ReasoningMessageStart
	// ReasoningMessageContent 构造推理消息内容增量事件。
	ReasoningMessageContent = stream.ReasoningMessageContent
	// ReasoningMessageEnd 构造推理消息结束事件。
	ReasoningMessageEnd = stream.ReasoningMessageEnd
	// ToolCallStart 构造工具调用开始事件。
	ToolCallStart = stream.ToolCallStart
	// ToolCallArgs 构造工具参数增量事件。
	ToolCallArgs = stream.ToolCallArgs
	// ToolCallEnd 构造工具调用结束事件。
	ToolCallEnd = stream.ToolCallEnd
	// ToolCallResult 构造工具调用结果事件。
	ToolCallResult = stream.ToolCallResult
	// ToolOutputDelta 构造工具输出增量事件。
	ToolOutputDelta = stream.ToolOutputDelta
	// IsToolOutputDelta 判断事件是否为工具输出增量。
	IsToolOutputDelta = stream.IsToolOutputDelta
	// ToolOutputDeltaFields 解析工具输出增量字段。
	ToolOutputDeltaFields = stream.ToolOutputDeltaFields
	// IsTextContent 判断事件是否为文本内容增量。
	IsTextContent = stream.IsTextContent
	// AppendTextDelta 追加文本增量。
	AppendTextDelta = stream.AppendTextDelta
	// MergeToolOutputDelta 合并工具输出增量。
	MergeToolOutputDelta = stream.MergeToolOutputDelta
	// NewCoalesceSink 创建文本合并 Sink。
	NewCoalesceSink = stream.NewCoalesceSink
	// NewOutputCoalesceSink 创建工具输出合并 Sink。
	NewOutputCoalesceSink = stream.NewOutputCoalesceSink
)

// CoalesceSinkClose 关闭文本合并 Sink（类型断言辅助）。
func CoalesceSinkClose(s Sink) {
	if c, ok := s.(*CoalesceSink); ok {
		c.Close()
	}
}

// OutputCoalesceSinkClose 关闭工具输出合并 Sink。
func OutputCoalesceSinkClose(s Sink) {
	if c, ok := s.(*OutputCoalesceSink); ok {
		c.Close()
	}
}

// BuildCoalescedSink 返回带文本/工具输出合并的 Sink 与关闭函数。
func BuildCoalescedSink(base Sink, textInterval, outputInterval time.Duration) (Sink, func()) {
	coalescedText := NewCoalesceSink(base, "", textInterval)
	coalesced := NewOutputCoalesceSink(coalescedText, "", outputInterval)
	return coalesced, func() {
		OutputCoalesceSinkClose(coalesced)
		CoalesceSinkClose(coalescedText)
	}
}

// PublishEvent 经 Sink 发布单条事件。
func PublishEvent(ctx context.Context, sink Sink, ev Event) error {
	if sink == nil {
		return nil
	}
	return sink.Publish(ctx, ev)
}
