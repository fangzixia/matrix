package query

import (
	"context"
	"fmt"
	"matrix/internal/ai/audit"
	"matrix/internal/ai/llm"
	"matrix/internal/ai/stream"
	"matrix/internal/ai/tools"
	"matrix/internal/platform/logging"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// logLinePrefix 返回非空的日志/事件行前缀（含尾部空格），用于区分父子 Agent 的 TAOR 循环。
func logLinePrefix(cfg Config) string {
	s := strings.TrimSpace(cfg.LogPrefix)
	if s == "" {
		return ""
	}
	return "[" + s + "] "
}

// sessionID 返回或生成当前会话 ID。
func sessionID(cfg Config) string {
	if cfg.SessionID != "" {
		return cfg.SessionID
	}
	return uuid.NewString()
}

// RunSession 启动 TAOR 会话，经 sink 推送 SDK 风格过程消息。
func RunSession(ctx context.Context, cfg Config, sink StreamSink) Result {
	if sink == nil {
		sink = stream.NopSink{}
	}
	if cfg.SessionID == "" {
		cfg.SessionID = sessionID(cfg)
	}
	ctx = logging.With(ctx, logging.Fields{
		logging.FieldSessionID: cfg.SessionID,
		logging.FieldComponent: auditComponent(cfg),
	})
	start := time.Now()
	result := queryLoop(ctx, cfg, sink)
	publishResult(ctx, cfg.SessionID, sink, result, start)
	return result
}

// auditComponent 返回审计日志组件名。
func auditComponent(cfg Config) string {
	if strings.TrimSpace(cfg.LogPrefix) != "" {
		return "coordinator:" + cfg.LogPrefix
	}
	return "query"
}

// publishResult 推送 TAOR 循环最终结果。
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
	case StopModelError:
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

// publish 将流式消息推送到 Sink。
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
		prepareHistoryForRequest(ctx, cfg, &s.messages)
		if cfg.MaxTurns > 0 && s.turnCount > cfg.MaxTurns {
			logging.InfoCtx(ctx, "loop: max turns reached", "turns", s.turnCount)
			return Result{StopReason: StopMaxTurns, TurnCount: s.turnCount, Messages: s.messages}
		}
		trans := transitionStr(s.transition)
		summary := fmt.Sprintf("%s第 %d 轮（跃迁: %s）", logLinePrefix(cfg), s.turnCount, trans)
		publish(ctx, sink, stream.TurnProgress(sid, s.turnCount, trans, summary))
		ctx = logging.With(ctx, logging.Fields{
			logging.FieldSessionID: sid,
			logging.FieldTurn:      fmt.Sprint(s.turnCount),
			logging.FieldComponent: auditComponent(cfg),
		})
		if cfg.Audit != nil {
			cfg.Audit.Emit("turn.iteration", s.turnCount, auditComponent(cfg), map[string]any{
				"transition":    trans,
				"message_count": len(s.messages),
				"log_prefix":    cfg.LogPrefix,
			})
		}
		logging.InfoCtx(ctx, "loop: iteration",
			"log_prefix", cfg.LogPrefix,
			"message_count", len(s.messages),
			"transition", trans,
		)
		turn, err := think(ctx, cfg, s.turnCount, s.messages, sink)
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
				logging.InfoCtx(ctx, "loop: stop hook blocking", "error", blockingErr)
				s = state{
					messages: append(s.messages, Message{
						Role:    RoleUser,
						Content: "[stop_hook] " + blockingErr,
					}),
					turnCount:  s.turnCount,
					transition: new(TransitionStopHookBlocking),
				}
				continue
			}
			prevLen := len(s.messages)
			s.messages = drainAsyncResults(cfg, s.turnCount, s.messages, cfg.AsyncResults, cfg.ContextPolicy.MaxAsyncResultRunes)
			if len(s.messages) > prevLen {
				logging.InfoCtx(ctx, "loop: async results drained", "new_messages", len(s.messages)-prevLen)
				continue
			}
			if cfg.HasPendingAsync != nil && cfg.HasPendingAsync() {
				logging.InfoCtx(ctx, "loop: waiting for async sub-agents")
				select {
				case <-ctx.Done():
					return Result{StopReason: StopAborted, TurnCount: s.turnCount,
						Err: ctx.Err(), Messages: s.messages}
				case msg := <-cfg.AsyncResults:
					emitAsyncAudit(cfg, s.turnCount, msg)
					s.messages = append(s.messages, truncateAsyncMessage(msg, cfg.ContextPolicy.MaxAsyncResultRunes))
					s.messages = drainAsyncResults(cfg, s.turnCount, s.messages, cfg.AsyncResults, cfg.ContextPolicy.MaxAsyncResultRunes)
					logging.InfoCtx(ctx, "loop: async result injected", "message_count", len(s.messages))
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
		toolResults := act(ctx, cfg, s.turnCount, turn.ToolCalls, sink)
		observeMsgs := observe(ctx, cfg, s.turnCount, toolResults)
		allMsgs := append(s.messages, observeMsgs...)
		allMsgs = drainAsyncResults(cfg, s.turnCount, allMsgs, cfg.AsyncResults, cfg.ContextPolicy.MaxAsyncResultRunes)
		s = state{
			messages:   allMsgs,
			turnCount:  s.turnCount + 1,
			transition: new(TransitionNextTurn),
		}
		logging.InfoCtx(ctx, "loop: turn completed",
			"completed_turn", s.turnCount-1,
			"tool_calls", len(turn.ToolCalls),
		)
	}
}

// emitAsyncAudit 记录异步子 Agent 相关审计事件。
func emitAsyncAudit(cfg Config, turn int, msg Message) {
	if cfg.Audit == nil {
		return
	}
	cfg.Audit.Emit("async.result", turn, auditComponent(cfg), map[string]any{
		"result_preview": audit.Preview(msg.Content, 500),
	})
}

// drainAsyncResults 非阻塞消费异步 Worker 结果通道。
func drainAsyncResults(cfg Config, turn int, msgs []Message, ch <-chan Message, maxRunes int) []Message {
	if ch == nil {
		return msgs
	}
	for {
		select {
		case msg := <-ch:
			emitAsyncAudit(cfg, turn, msg)
			msgs = append(msgs, truncateAsyncMessage(msg, maxRunes))
		default:
			return msgs
		}
	}
}

// toolCallsToBlocks 将工具调用列表转换为 LLM 消息块。
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

// think 执行 TAOR 循环的 T（思考）阶段：调用 LLM 并流式接收响应。
func think(
	ctx context.Context,
	cfg Config,
	turn int,
	history []Message,
	sink StreamSink,
) (*llm.AssistantTurn, error) {
	sid := cfg.SessionID
	tokensEst := estimateRequestTokens(cfg, history)
	if cfg.Audit != nil {
		cfg.Audit.Emit("turn.llm_request", turn, auditComponent(cfg), map[string]any{
			"model":       cfg.Model,
			"history_len": len(history),
			"tokens_est":  tokensEst,
		})
	}
	logging.InfoCtx(ctx, "loop: llm request", "model", cfg.Model, "history_len", len(history), "tokens_est", tokensEst)
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
	if cfg.Audit != nil {
		cfg.Audit.Emit("turn.llm_response", turn, auditComponent(cfg), map[string]any{
			"finish_reason":    finalTurn.FinishReason,
			"tool_calls":       len(finalTurn.ToolCalls),
			"content_preview":  audit.Preview(finalTurn.Content, 300),
			"thinking_preview": audit.Preview(finalTurn.Thinking, 300),
		})
	}
	logging.InfoCtx(ctx, "loop: llm response",
		"finish_reason", finalTurn.FinishReason,
		"tool_calls", len(finalTurn.ToolCalls),
		"content_len", len(finalTurn.Content),
	)
	return finalTurn, nil
}

// act 执行 TAOR 循环的 A（行动）阶段：执行工具调用。
func act(
	ctx context.Context,
	cfg Config,
	turn int,
	toolCalls []llm.ToolCall,
	sink StreamSink,
) []tools.Result {
	if cfg.Registry == nil || len(toolCalls) == 0 {
		return nil
	}
	sid := cfg.SessionID
	logging.InfoCtx(ctx, "loop: tool execution", "count", len(toolCalls))
	for _, tc := range toolCalls {
		if cfg.Audit != nil {
			cfg.Audit.EmitWithTool("turn.tool_call", turn, auditComponent(cfg), tc.ID, map[string]any{
				"tool_name":     tc.Function.Name,
				"input_preview": audit.Preview(tc.Function.Arguments, 500),
			})
		}
	}
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
		logging.InfoCtx(ctx, "loop: tool result", "tool_name", r.ToolName, "is_error", r.IsError)
	}
	return results
}

// observe 执行 TAOR 循环的 O（观察）阶段：将工具结果打包为用户消息。
func observe(ctx context.Context, cfg Config, turn int, results []tools.Result) []Message {
	msgs := make([]Message, 0, len(results))
	for _, r := range results {
		out := r.Output
		if cfg.MaxToolResultRunes > 0 {
			out = truncateRunes(out, cfg.MaxToolResultRunes)
		}
		if cfg.Audit != nil {
			cfg.Audit.EmitWithTool("turn.tool_result", turn, auditComponent(cfg), r.ToolCallID, map[string]any{
				"tool_name":      r.ToolName,
				"is_error":       r.IsError,
				"output_preview": audit.Preview(r.Output, 500),
			})
		}
		msgs = append(msgs, Message{
			Role:       RoleTool,
			Content:    out,
			ToolCallID: r.ToolCallID,
			ToolName:   r.ToolName,
			IsError:    r.IsError,
		})
	}
	logging.InfoCtx(ctx, "loop: observe done", "result_count", len(msgs))
	return msgs
}

// truncateRunes 按 Unicode rune 截断字符串。
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

// report 执行 TAOR 循环的 R（汇报）阶段：Stop Hook 检查并输出最终答案。
func report(history []Message, stopHook func([]Message) string) string {
	if stopHook == nil {
		return ""
	}
	return stopHook(history)
}

// buildChatMessages 将内部 Message 列表转换为 LLM API 消息格式。
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

// transitionStr 将跃迁原因枚举格式化为字符串。
func transitionStr(t *TransitionReason) string {
	if t == nil {
		return "initial"
	}
	return string(*t)
}
