package query

import (
	"context"
	"encoding/json"
	"fmt"
	"matrix/internal/logger"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"matrix/internal/llm"
	"matrix/internal/stream"
	"matrix/internal/tools"
)

// maxOutputTokensRecoveryLimit 是单轮内允许触发 max_output_tokens 恢复的最大次数。
const maxOutputTokensRecoveryLimit = 3

// logLinePrefix 返回非空的日志/事件行前缀（含尾部空格），用于区分父子 Agent 的 TAOR 循环。
func logLinePrefix(cfg Config) string {
	s := strings.TrimSpace(cfg.LogPrefix)
	if s == "" {
		return ""
	}
	return "[" + s + "] "
}

func sessionID(cfg Config) string {
	if cfg.SessionID != "" {
		return cfg.SessionID
	}
	return uuid.NewString()
}

// drainAsyncResults 非阻塞地消费 ch 中所有已就绪的消息，
// 将其作为 user 消息追加到 msgs 并返回更新后的切片。
func drainAsyncResults(msgs []Message, ch <-chan Message, maxRunes int) []Message {
	if ch == nil {
		return msgs
	}
	for {
		select {
		case msg := <-ch:
			msgs = append(msgs, truncateAsyncMessage(msg, maxRunes))
		default:
			return msgs
		}
	}
}

// Run 启动 TAOR 会话并阻塞直到循环终止（不推送流式消息）。
func Run(ctx context.Context, cfg Config) Result {
	return RunSession(ctx, cfg, stream.NopSink{})
}

// RunSession 启动 TAOR 会话，经 sink 推送 SDK 风格过程消息。
func RunSession(ctx context.Context, cfg Config, sink StreamSink) Result {
	if sink == nil {
		sink = stream.NopSink{}
	}
	if cfg.SessionID == "" {
		cfg.SessionID = sessionID(cfg)
	}
	start := time.Now()
	result := queryLoop(ctx, cfg, sink)
	publishResult(ctx, cfg.SessionID, sink, result, start)
	return result
}

func publishResult(ctx context.Context, sid string, sink StreamSink, r Result, start time.Time) {
	if sink == nil {
		return
	}
	dur := time.Since(start)
	switch r.StopReason {
	case StopCompleted:
		_ = sink.Publish(ctx, stream.ResultSuccessMsg(sid, r.Answer, string(r.StopReason), r.TurnCount, dur))
	case StopMaxTurns:
		_ = sink.Publish(ctx, stream.ResultMaxTurns(sid, r.TurnCount, dur))
	case StopAborted:
		errMsg := "已取消"
		if r.Err != nil {
			errMsg = r.Err.Error()
		}
		_ = sink.Publish(ctx, stream.ResultErrorMsg(sid, string(r.StopReason), errMsg, r.TurnCount, dur))
	case StopModelError, StopHookPause:
		errMsg := "模型错误"
		if r.Err != nil {
			errMsg = r.Err.Error()
		}
		_ = sink.Publish(ctx, stream.ResultErrorMsg(sid, string(r.StopReason), errMsg, r.TurnCount, dur))
	default:
		errMsg := ""
		if r.Err != nil {
			errMsg = r.Err.Error()
		}
		_ = sink.Publish(ctx, stream.ResultErrorMsg(sid, string(r.StopReason), errMsg, r.TurnCount, dur))
	}
}

func publish(ctx context.Context, sink StreamSink, msg stream.Message) {
	if sink == nil {
		return
	}
	_ = sink.Publish(ctx, msg)
}

// queryLoop 是 TAOR 循环的核心 for{} 状态机。
func queryLoop(ctx context.Context, cfg Config, sink StreamSink) Result {
	sid := cfg.SessionID
	s := state{
		messages:  append([]Message(nil), cfg.InitialMessages...),
		turnCount: 1,
	}

	for {
		if err := ctx.Err(); err != nil {
			return Result{StopReason: StopAborted, TurnCount: s.turnCount, Err: err, Messages: s.messages}
		}

		prepareHistoryForRequest(cfg, &s.messages)

		if cfg.MaxTurns > 0 && s.turnCount > cfg.MaxTurns {
			logger.Info("loop: 达到最大轮次", "turns", s.turnCount)
			return Result{StopReason: StopMaxTurns, TurnCount: s.turnCount, Messages: s.messages}
		}

		trans := transitionStr(s.transition)
		summary := fmt.Sprintf("%s第 %d 轮（跃迁: %s）", logLinePrefix(cfg), s.turnCount, trans)
		publish(ctx, sink, stream.TurnProgress(sid, s.turnCount, trans, summary))

		logger.Info("loop: 循环迭代",
			"标签", cfg.LogPrefix,
			"轮次", s.turnCount,
			"消息数", len(s.messages),
			"跃迁", trans,
		)

		turn, err := think(ctx, cfg, s.messages, sink)
		if err != nil {
			return Result{StopReason: StopModelError, TurnCount: s.turnCount, Err: err, Messages: s.messages}
		}

		assistantMsg := Message{
			Role:      RoleAssistant,
			Content:   turn.Content,
			Thinking:  turn.Thinking,
			ToolCalls: turn.ToolCalls,
		}
		s.messages = append(s.messages, assistantMsg)

		toolBlocks := toolCallsToBlocks(turn.ToolCalls)
		publish(ctx, sink, stream.Assistant(sid, turn.Content, turn.Thinking, toolBlocks, turn.FinishReason))

		needsFollowUp := len(turn.ToolCalls) > 0

		if !needsFollowUp {
			if blockingErr := report(s.messages, cfg.StopHook); blockingErr != "" {
				logger.Info("loop: stop hook 阻塞，重新进入循环", "error", blockingErr)
				reason := TransitionStopHookBlocking
				s = state{
					messages: append(s.messages, Message{
						Role:    RoleUser,
						Content: "[stop_hook] " + blockingErr,
					}),
					turnCount:  s.turnCount,
					transition: &reason,
				}
				continue
			}

			prevLen := len(s.messages)
			s.messages = drainAsyncResults(s.messages, cfg.AsyncResults, cfg.ContextPolicy.MaxAsyncResultRunes)

			if len(s.messages) > prevLen {
				logger.Info("loop: 收到异步 Agent 结果，继续循环",
					"新消息数", len(s.messages)-prevLen)
				continue
			}

			if cfg.HasPendingAsync != nil && cfg.HasPendingAsync() {
				logger.Info("loop: 等待异步子 Agent 完成...")
				select {
				case <-ctx.Done():
					return Result{StopReason: StopAborted, TurnCount: s.turnCount,
						Err: ctx.Err(), Messages: s.messages}
				case msg := <-cfg.AsyncResults:
					s.messages = append(s.messages, truncateAsyncMessage(msg, cfg.ContextPolicy.MaxAsyncResultRunes))
					s.messages = drainAsyncResults(s.messages, cfg.AsyncResults, cfg.ContextPolicy.MaxAsyncResultRunes)
					logger.Info("loop: 异步结果已注入，继续循环",
						"当前消息数", len(s.messages))
					continue
				}
			}

			return Result{
				StopReason: StopCompleted,
				TurnCount:  s.turnCount,
				Answer:     turn.Content,
				Messages:   s.messages,
			}
		}

		toolResults := act(ctx, cfg, turn.ToolCalls, sink)
		observeMsgs := observe(toolResults, cfg)

		next := TransitionNextTurn
		allMsgs := append(s.messages, observeMsgs...)
		allMsgs = drainAsyncResults(allMsgs, cfg.AsyncResults, cfg.ContextPolicy.MaxAsyncResultRunes)
		s = state{
			messages:   allMsgs,
			turnCount:  s.turnCount + 1,
			transition: &next,
		}

		logger.Info("loop: 本轮完成",
			"轮次", s.turnCount-1,
			"工具调用数", len(turn.ToolCalls),
		)
	}
}

func toolCallsToBlocks(calls []llm.ToolCall) []stream.ToolUseBlock {
	out := make([]stream.ToolUseBlock, 0, len(calls))
	for _, tc := range calls {
		out = append(out, stream.ToolUseBlock{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: tc.Function.Arguments,
		})
	}
	return out
}

func think(
	ctx context.Context,
	cfg Config,
	history []Message,
	sink StreamSink,
) (*llm.AssistantTurn, error) {
	sid := cfg.SessionID
	logger.Info("loop: T — 调用模型", "model", cfg.Model, "历史长度", len(history))

	req := llm.ChatRequest{
		Model:     cfg.Model,
		Messages:  buildChatMessages(cfg.SystemPrompt, history),
		MaxTokens: cfg.MaxTokens,
	}
	if cfg.Registry != nil {
		req.Tools = cfg.Registry.LLMTools()
	}

	publish(ctx, sink, stream.MessageStart(sid))

	var finalTurn *llm.AssistantTurn
	blockIndex := 0
	for ev := range cfg.LLM.Stream(ctx, req) {
		if ev.Err != nil {
			return nil, fmt.Errorf("loop: 模型错误: %w", ev.Err)
		}
		if ev.TextDelta != "" {
			publish(ctx, sink, stream.TextDelta(sid, ev.TextDelta, blockIndex))
		}
		if ev.ThinkingDelta != "" {
			publish(ctx, sink, stream.ThinkingDelta(sid, ev.ThinkingDelta, blockIndex))
		}
		if ev.Turn != nil {
			finalTurn = ev.Turn
		}
	}

	publish(ctx, sink, stream.MessageStop(sid))

	if finalTurn == nil {
		return nil, fmt.Errorf("loop: 模型流结束但未收到完整 turn")
	}

	logger.Info("loop: T — 完成",
		"finish_reason", finalTurn.FinishReason,
		"工具调用数", len(finalTurn.ToolCalls),
		"文本长度", len(finalTurn.Content),
	)
	return finalTurn, nil
}

func act(
	ctx context.Context,
	cfg Config,
	toolCalls []llm.ToolCall,
	sink StreamSink,
) []tools.Result {
	if cfg.Registry == nil || len(toolCalls) == 0 {
		return nil
	}
	sid := cfg.SessionID
	logger.Info("loop: A — 执行工具", "数量", len(toolCalls))

	onProgress := func(toolUseID string, data stream.ToolProgressData) {
		msg := stream.Message{
			Type:      stream.TypeProgress,
			SessionID: sid,
			UUID:      uuid.NewString(),
			ToolUseID: toolUseID,
			Data:      &data,
		}
		publish(ctx, sink, msg)
	}

	results := tools.RunTools(ctx, toolCalls, cfg.Registry, cfg.CanUseTool, onProgress)
	for _, r := range results {
		logger.Info("loop: A — 工具结果", "工具", r.ToolName, "错误", r.IsError)
	}
	return results
}

func observe(results []tools.Result, cfg Config) []Message {
	msgs := make([]Message, 0, len(results))
	for _, r := range results {
		out := r.Output
		if cfg.MaxToolResultRunes > 0 {
			out = truncateRunes(out, cfg.MaxToolResultRunes)
		}
		msgs = append(msgs, Message{
			Role:       RoleTool,
			Content:    out,
			ToolCallID: r.ToolCallID,
			ToolName:   r.ToolName,
			IsError:    r.IsError,
		})
	}
	logger.Info("loop: O — 观察完成", "结果数", len(msgs))
	return msgs
}

func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}
	return string(runes) + "\n…（工具输出已按 MaxToolResultRunes 截断）"
}

func report(history []Message, stopHook func([]Message) string) string {
	if stopHook == nil {
		return ""
	}
	return stopHook(history)
}

func buildChatMessages(systemPrompt string, history []Message) []llm.ChatMessage {
	var msgs []llm.ChatMessage

	if systemPrompt != "" {
		msgs = append(msgs, llm.ChatMessage{
			Role:    string(RoleSystem),
			Content: systemPrompt,
		})
	}

	for _, m := range history {
		switch m.Role {
		case RoleUser:
			msgs = append(msgs, llm.ChatMessage{
				Role:    "user",
				Content: m.Content,
			})

		case RoleAssistant:
			msg := llm.ChatMessage{
				Role:             "assistant",
				Content:          m.Content,
				ReasoningContent: m.Thinking,
			}
			if len(m.ToolCalls) > 0 {
				msg.ToolCalls = m.ToolCalls
				msg.Content = ""
			}
			msgs = append(msgs, msg)

		case RoleTool:
			msgs = append(msgs, llm.ChatMessage{
				Role:       "tool",
				Content:    m.Content,
				ToolCallID: m.ToolCallID,
				Name:       m.ToolName,
			})

		case RoleSystem:
			msgs = append(msgs, llm.ChatMessage{
				Role:    "system",
				Content: m.Content,
			})
		}
	}
	return msgs
}

func transitionStr(t *TransitionReason) string {
	if t == nil {
		return "initial"
	}
	return string(*t)
}

func toolCallSummary(tc llm.ToolCall) string {
	args := tc.Function.Arguments
	return fmt.Sprintf("%s(%s)", tc.Function.Name, args)
}

func formatToolResults(results []tools.Result) string {
	var sb strings.Builder
	for _, r := range results {
		status := "ok"
		if r.IsError {
			status = "error"
		}
		preview := r.Output
		sb.WriteString(fmt.Sprintf("  %s[%s]: %s\n", r.ToolName, status, preview))
	}
	return sb.String()
}

func marshalArgs(args map[string]any) string {
	b, _ := json.Marshal(args)
	return string(b)
}

var _ = toolCallSummary
var _ = formatToolResults
var _ = marshalArgs
