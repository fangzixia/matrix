package query

import (
	"context"
	"fmt"
	"matrix/ai/llm"
	"matrix/ai/stream"
	"matrix/ai/util"
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

// RunSession 启动 TAOR 会话，经 sink 推送 AG-UI 事件序列。
func RunSession(ctx context.Context, cfg Config, sink StreamSink) Result {
	if err := cfg.Validate(); err != nil {
		return Result{StopReason: StopModelError, Err: err}
	}
	if sink == nil {
		sink = stream.NopSink{}
	}
	meta := sessionMeta(cfg)
	if meta.RunID == "" {
		meta.RunID = stream.NewRunID()
	}
	cfg.RunID = meta.RunID
	cfg.ThreadID = meta.ThreadID
	if cfg.SessionID == "" {
		cfg.SessionID = meta.RunID
	}
	ctx = stream.WithMeta(ctx, meta)
	ctx = withLogFields(ctx, LogFields{
		logFieldSessionID: cfg.SessionID,
		logFieldComponent: auditComponent(cfg),
	})
	publish(ctx, sink, stream.RunStarted(meta))
	start := time.Now()
	result := queryLoop(ctx, cfg, sink, meta)
	publishResult(ctx, meta, sink, result, start)
	return result
}

func sessionMeta(cfg Config) stream.Meta {
	return stream.Meta{
		ThreadID:    cfg.ThreadID,
		RunID:       cfg.RunID,
		ParentRunID: cfg.ParentRunID,
	}
}

// auditComponent 返回审计日志组件名。
func auditComponent(cfg Config) string {
	if strings.TrimSpace(cfg.LogPrefix) != "" {
		return "coordinator:" + cfg.LogPrefix
	}
	return "query"
}

// logAgentOutcome 记录 TAOR 循环终态到 agent.log。
func logAgentOutcome(ctx context.Context, r Result) {
	args := []any{"stop_reason", r.StopReason, "turns", r.TurnCount}
	if r.Err != nil {
		args = append(args, "error", r.Err.Error())
	}
	agentLog(ctx, "loop: 结束", args...)
}

func returnOutcome(ctx context.Context, r Result) Result {
	logAgentOutcome(ctx, r)
	return r
}

// publishResult 推送 TAOR 循环最终结果（RUN_FINISHED / RUN_ERROR）。
func publishResult(ctx context.Context, meta stream.Meta, sink StreamSink, r Result, start time.Time) {
	if sink == nil {
		return
	}
	var ev stream.Event
	switch r.StopReason {
	case StopCompleted:
		ev = stream.RunFinished(meta, map[string]any{
			"stopReason": string(r.StopReason),
			"output":     r.Answer,
			"numTurns":   r.TurnCount,
			"durationMs": time.Since(start).Milliseconds(),
		})
	case StopMaxTurns:
		ev = stream.RunError(meta, "已达到最大轮次上限")
	case StopAborted:
		errMsg := "已取消"
		if r.Err != nil {
			errMsg = r.Err.Error()
		}
		ev = stream.RunError(meta, errMsg)
	case StopModelError:
		errMsg := "模型错误"
		if r.Err != nil {
			errMsg = r.Err.Error()
		}
		ev = stream.RunError(meta, errMsg)
	default:
		errMsg := ""
		if r.Err != nil {
			errMsg = r.Err.Error()
		}
		if errMsg == "" {
			ev = stream.RunFinished(meta, map[string]any{
				"stopReason": string(r.StopReason),
				"numTurns":   r.TurnCount,
				"durationMs": time.Since(start).Milliseconds(),
			})
		} else {
			ev = stream.RunError(meta, errMsg)
		}
	}
	if ev == nil {
		return
	}
	if err := sink.Publish(ctx, ev); err != nil {
		agentLog(ctx, "stream: publish 失败", "error", err, "run_id", meta.RunID, "type", stream.EventType(ev))
	}
}

// publish 将 AG-UI 事件推送到 Sink。
func publish(ctx context.Context, sink StreamSink, ev stream.Event) {
	if sink == nil || ev == nil {
		return
	}
	if err := sink.Publish(ctx, ev); err != nil {
		agentLog(ctx, "stream: publish 失败", "error", err, "type", stream.EventType(ev))
	}
}

// queryLoop 是 TAOR 循环的核心 for{} 状态机。
func queryLoop(ctx context.Context, cfg Config, sink StreamSink, meta stream.Meta) Result {
	s := state{
		messages:  append([]Message(nil), cfg.InitialMessages...),
		turnCount: 1,
	}
	messageID := stream.NewMessageID()
	for {
		if err := ctx.Err(); err != nil {
			return returnOutcome(ctx, Result{StopReason: StopAborted, TurnCount: s.turnCount, Err: err, Messages: s.messages})
		}
		prepareHistoryForRequest(ctx, cfg, &s.messages)
		if cfg.MaxTurns > 0 && s.turnCount > cfg.MaxTurns {
			return returnOutcome(ctx, Result{StopReason: StopMaxTurns, TurnCount: s.turnCount, Messages: s.messages})
		}
		trans := transitionStr(s.transition)
		summary := TurnThinkingLabel(s.turnCount, trans)
		publish(ctx, sink, stream.StepStarted(s.turnCount))
		ctx = withLogFields(ctx, LogFields{
			logFieldSessionID: cfg.SessionID,
			logFieldTurn:      fmt.Sprint(s.turnCount),
			logFieldComponent: auditComponent(cfg),
		})
		if cfg.Audit != nil {
			cfg.Audit.Emit("turn.iteration", s.turnCount, auditComponent(cfg), map[string]any{
				"transition":    trans,
				"message_count": len(s.messages),
				"log_prefix":    cfg.LogPrefix,
				"summary":       summary,
			})
		}
		agentLog(ctx, "loop: 决策",
			"log_prefix", cfg.LogPrefix,
			"activity", summary,
			"message_count", len(s.messages),
			"transition", trans,
		)
		turn, err := think(ctx, cfg, s.turnCount, s.messages, sink, messageID)
		if err != nil {
			publish(ctx, sink, stream.StepFinished(s.turnCount))
			return returnOutcome(ctx, Result{StopReason: StopModelError, TurnCount: s.turnCount, Err: err, Messages: s.messages})
		}
		assistantMsg := Message{
			Role:      RoleAssistant,
			Content:   turn.Content,
			Thinking:  turn.Thinking,
			ToolCalls: turn.ToolCalls,
		}
		s.messages = append(s.messages, assistantMsg)
		publish(ctx, sink, stream.StepFinished(s.turnCount))
		if len(turn.ToolCalls) == 0 {
			if done, finished := tryFinishTurn(ctx, cfg, &s, turn); finished {
				return returnOutcome(ctx, done)
			}
			messageID = stream.NewMessageID()
			continue
		}
		toolResults := act(ctx, cfg, s.turnCount, turn.ToolCalls, sink, messageID)
		observeMsgs := observe(ctx, cfg, s.turnCount, toolResults)
		allMsgs := append(s.messages, observeMsgs...)
		allMsgs = drainAsyncResults(cfg, s.turnCount, allMsgs, cfg.AsyncResults, cfg.ContextPolicy.MaxAsyncResultRunes)
		s = state{
			messages:   allMsgs,
			turnCount:  s.turnCount + 1,
			transition: new(TransitionNextTurn),
		}
		messageID = stream.NewMessageID()
		agentLog(ctx, "loop: 步骤完成",
			"activity", SummarizeToolCalls(turn.ToolCalls),
			"tool_calls", len(turn.ToolCalls),
		)
	}
}

// tryFinishTurn 处理无 tool call 时的 stop hook、async drain 与阻塞等待。
// finished 为 true 时 done 为终态结果；false 时调用方应 continue 主循环。
func tryFinishTurn(ctx context.Context, cfg Config, s *state, turn *llm.AssistantTurn) (done Result, finished bool) {
	if blockingErr := report(s.messages, cfg.StopHook); blockingErr != "" {
		agentLog(ctx, "loop: Stop Hook 阻塞", "error", blockingErr)
		*s = state{
			messages: append(s.messages, Message{
				Role:    RoleUser,
				Content: "[stop_hook] " + blockingErr,
			}),
			turnCount:  s.turnCount,
			transition: new(TransitionStopHookBlocking),
		}
		return Result{}, false
	}
	prevLen := len(s.messages)
	s.messages = drainAsyncResults(cfg, s.turnCount, s.messages, cfg.AsyncResults, cfg.ContextPolicy.MaxAsyncResultRunes)
	if len(s.messages) > prevLen {
		agentLog(ctx, "loop: Worker 结果已消费", "new_messages", len(s.messages)-prevLen)
		return Result{}, false
	}
	if cfg.HasPendingAsync != nil && cfg.HasPendingAsync() {
		agentLog(ctx, "loop: 等待 Worker", "activity", LabelWaitingWorkers)
		select {
		case <-ctx.Done():
			return Result{
				StopReason: StopAborted,
				TurnCount:  s.turnCount,
				Err:        ctx.Err(),
				Messages:   s.messages,
			}, true
		case msg := <-cfg.AsyncResults:
			emitAsyncAudit(cfg, s.turnCount, msg)
			s.messages = append(s.messages, truncateAsyncMessage(msg, cfg.ContextPolicy.MaxAsyncResultRunes))
			s.messages = drainAsyncResults(cfg, s.turnCount, s.messages, cfg.AsyncResults, cfg.ContextPolicy.MaxAsyncResultRunes)
			agentLog(ctx, "loop: Worker 结果注入",
				"activity", AsyncResultLabel(msg.Content),
				"message_count", len(s.messages),
			)
			return Result{}, false
		}
	}
	return Result{
		StopReason: StopCompleted,
		TurnCount:  s.turnCount,
		Answer:     turn.Content,
		Messages:   s.messages,
	}, true
}

// emitAsyncAudit 记录异步子 Agent 相关审计事件。
func emitAsyncAudit(cfg Config, turn int, msg Message) {
	if cfg.Audit == nil {
		return
	}
	cfg.Audit.Emit("async.result", turn, auditComponent(cfg), map[string]any{
		"result_preview": PreviewText(msg.Content, 500),
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

// think 执行 TAOR 循环的 T（思考）阶段：调用 LLM 并流式接收响应。
func think(
	ctx context.Context,
	cfg Config,
	turn int,
	history []Message,
	sink StreamSink,
	messageID string,
) (*llm.AssistantTurn, error) {
	tokensEst := estimateRequestTokens(cfg, history)
	if cfg.Audit != nil {
		cfg.Audit.Emit("turn.llm_request", turn, auditComponent(cfg), map[string]any{
			"model":       cfg.Model,
			"history_len": len(history),
			"tokens_est":  tokensEst,
		})
	}
	req := llm.ChatRequest{
		Model:     cfg.Model,
		Messages:  buildChatMessages(cfg.SystemPrompt, history),
		MaxTokens: cfg.MaxTokens,
	}
	if cfg.Registry != nil {
		req.Tools = cfg.Registry.LLMTools()
	}
	publish(ctx, sink, stream.TextMessageStart(messageID, ""))
	publish(ctx, sink, stream.ReasoningMessageStart(messageID))
	var finalTurn *llm.AssistantTurn
	pendingToolIDs := make(map[int]string)
	startedTools := make(map[string]bool)
	for ev := range cfg.LLM.Stream(ctx, req) {
		if ev.Err != nil {
			agentLog(ctx, "loop: 模型错误", "error", ev.Err.Error())
			return nil, fmt.Errorf("loop: 模型错误: %w", ev.Err)
		}
		if ev.TextDelta != "" {
			publish(ctx, sink, stream.TextMessageContent(messageID, ev.TextDelta))
		}
		if ev.ThinkingDelta != "" {
			publish(ctx, sink, stream.ReasoningMessageContent(messageID, ev.ThinkingDelta))
		}
		if ev.ToolCallDelta != nil {
			d := ev.ToolCallDelta
			toolUseID := resolveToolUseID(d, pendingToolIDs)
			if d.Name != "" && !startedTools[toolUseID] {
				publish(ctx, sink, stream.ToolCallStart(toolUseID, d.Name))
				startedTools[toolUseID] = true
			}
			if d.ArgumentsDelta != "" {
				publish(ctx, sink, stream.ToolCallArgs(toolUseID, d.ArgumentsDelta))
			}
		}
		if ev.Turn != nil {
			finalTurn = ev.Turn
		}
	}
	publish(ctx, sink, stream.ReasoningMessageEnd(messageID))
	publish(ctx, sink, stream.TextMessageEnd(messageID))
	if finalTurn == nil {
		return nil, fmt.Errorf("loop: 模型流结束但未收到完整 turn")
	}
	if cfg.Audit != nil {
		cfg.Audit.Emit("turn.llm_response", turn, auditComponent(cfg), map[string]any{
			"finish_reason":    finalTurn.FinishReason,
			"tool_calls":       len(finalTurn.ToolCalls),
			"content_preview":  PreviewText(finalTurn.Content, 300),
			"thinking_preview": PreviewText(finalTurn.Thinking, 300),
		})
	}
	return finalTurn, nil
}

func resolveToolUseID(d *llm.ToolCallDelta, pending map[int]string) string {
	if d.ID != "" {
		pending[d.Index] = d.ID
		return d.ID
	}
	if existing, ok := pending[d.Index]; ok {
		return existing
	}
	id := fmt.Sprintf("pending-%d", d.Index)
	pending[d.Index] = id
	return id
}

// act 执行 TAOR 循环的 A（行动）阶段：执行工具调用。
func act(
	ctx context.Context,
	cfg Config,
	turn int,
	toolCalls []llm.ToolCall,
	sink StreamSink,
	messageID string,
) []util.Result {
	if cfg.Registry == nil || len(toolCalls) == 0 {
		return nil
	}
	for _, tc := range toolCalls {
		activityLabel := SummarizeSingleTool(tc.Function.Name, tc.Function.Arguments)
		agentLog(ctx, "loop: 工具执行",
			"tool_name", tc.Function.Name,
			"tool_call_id", tc.ID,
			"activity", activityLabel,
			"input", tc.Function.Arguments,
		)
		if cfg.Audit != nil {
			cfg.Audit.EmitWithTool("turn.tool_call", turn, auditComponent(cfg), tc.ID, map[string]any{
				"tool_name":     tc.Function.Name,
				"input_preview": PreviewText(tc.Function.Arguments, 500),
			})
		}
	}
	onProgress := func(ev stream.Event) {
		publish(ctx, sink, ev)
	}
	results := util.RunTools(ctx, toolCalls, cfg.Registry, cfg.CanUseTool, onProgress, messageID)
	for _, r := range results {
		agentLog(ctx, "loop: 工具结果",
			"tool_name", r.ToolName,
			"tool_call_id", r.ToolCallID,
			"is_error", r.IsError,
			"output", r.Output,
		)
	}
	return results
}

// observe 执行 TAOR 循环的 O（观察）阶段：将工具结果打包为用户消息。
func observe(ctx context.Context, cfg Config, turn int, results []util.Result) []Message {
	msgs := make([]Message, 0, len(results))
	for _, r := range results {
		out := r.Output
		if cfg.MaxToolResultRunes > 0 {
			out = TruncateRunes(out, cfg.MaxToolResultRunes)
		}
		if cfg.Audit != nil {
			cfg.Audit.EmitWithTool("turn.tool_result", turn, auditComponent(cfg), r.ToolCallID, map[string]any{
				"tool_name":      r.ToolName,
				"is_error":       r.IsError,
				"output_preview": PreviewText(r.Output, 500),
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
	agentLog(ctx, "loop: Observe 完成", "result_count", len(msgs))
	return msgs
}

// TruncateRunes 按 Unicode rune 截断字符串。
func TruncateRunes(s string, maxRunes int) string {
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
			if len(m.Attachments) == 0 {
				msgs = append(msgs, llm.ChatMessage{
					Role:    "user",
					Content: m.Content,
				})
			} else {
				msgs = append(msgs, llm.ChatMessage{
					Role:    "user",
					Content: buildUserContentParts(m),
				})
			}
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

// buildUserContentParts 将含附件的用户消息转换为 LLM 多模态内容块。
func buildUserContentParts(m Message) []llm.ContentPart {
	text := m.Content
	for _, att := range m.Attachments {
		if att.Type == "document" {
			if text != "" {
				text += "\n\n"
			}
			text += fmt.Sprintf("[附件: %s]\n%s", att.Name, att.Data)
		}
	}
	var parts []llm.ContentPart
	if text != "" {
		parts = append(parts, llm.ContentPart{Type: "text", Text: text})
	}
	for _, att := range m.Attachments {
		if att.Type != "image" {
			continue
		}
		mime := att.MimeType
		if mime == "" {
			mime = "image/png"
		}
		parts = append(parts, llm.ContentPart{
			Type: "image_url",
			ImageURL: &llm.ImageURL{
				URL: fmt.Sprintf("data:%s;base64,%s", mime, att.Data),
			},
		})
	}
	if len(parts) == 0 {
		return []llm.ContentPart{{Type: "text", Text: m.Content}}
	}
	return parts
}

// transitionStr 将跃迁原因枚举格式化为字符串。
func transitionStr(t *TransitionReason) string {
	if t == nil {
		return "initial"
	}
	return string(*t)
}
