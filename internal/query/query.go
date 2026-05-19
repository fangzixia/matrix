package query

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"matrix/internal/llm"
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

// drainAsyncResults 非阻塞地消费 ch 中所有已就绪的消息，
// 将其作为 user 消息追加到 msgs 并返回更新后的切片。
// ch 为 nil 时直接返回原切片。
// 对应 claude-code 的 notifyOnCompletion：Worker 结果以 user-role 消息注入父对话。
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

// Run 启动 TAOR 会话并阻塞直到循环终止，返回最终 [Result]。
//
// events 为可选参数；传 nil 则丢弃所有实时事件。
// Run 返回前 events channel 会被关闭。
func Run(ctx context.Context, cfg Config, events chan<- Event) Result {
	if events != nil {
		defer close(events)
	}
	return queryLoop(ctx, cfg, events)
}

// queryLoop 是 TAOR 循环的核心 for{} 状态机，对应 query.ts 的 queryLoop()。
// 每次迭代按顺序执行 Think → Act → Observe → Report 四个阶段。
func queryLoop(ctx context.Context, cfg Config, events chan<- Event) Result {
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
			slog.Info("loop: 达到最大轮次", "turns", s.turnCount)
			return Result{StopReason: StopMaxTurns, TurnCount: s.turnCount, Messages: s.messages}
		}

		emit(events, Event{
			Kind:  EventTurnStart,
			Delta: fmt.Sprintf("%s第 %d 轮（跃迁: %s）", logLinePrefix(cfg), s.turnCount, transitionStr(s.transition)),
		})

		slog.Info("loop: 循环迭代",
			"标签", cfg.LogPrefix,
			"轮次", s.turnCount,
			"消息数", len(s.messages),
			"跃迁", transitionStr(s.transition),
		)

		// T — Think：调用 LLM，流式接收 assistant 响应。
		turn, err := think(ctx, cfg, s.messages, events)
		if err != nil {
			return Result{StopReason: StopModelError, TurnCount: s.turnCount, Err: err, Messages: s.messages}
		}

		assistantMsg := Message{
			Role:      RoleAssistant,
			Content:   turn.Content,
			Thinking:  turn.Thinking, // [compat:deepseek/claude] 保存思考内容以便下轮回传
			ToolCalls: turn.ToolCalls,
		}
		s.messages = append(s.messages, assistantMsg)

		for _, tc := range turn.ToolCalls {
			emit(events, Event{
				Kind:       EventToolCall,
				ToolName:   tc.Function.Name,
				ToolCallID: tc.ID,
				ToolInput:  tc.Function.Arguments,
			})
		}

		needsFollowUp := len(turn.ToolCalls) > 0

		if !needsFollowUp {
			// R — Report：检查 Stop Hook，决定结束还是继续循环。
			if blockingErr := report(s.messages, cfg.StopHook); blockingErr != "" {
				slog.Info("loop: stop hook 阻塞，重新进入循环", "error", blockingErr)
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

			// 先非阻塞 drain 已就绪的异步结果。
			// 对应 claude-code：Worker 完成后以 user-role 消息注入 Coordinator 对话。
			prevLen := len(s.messages)
			s.messages = drainAsyncResults(s.messages, cfg.AsyncResults, cfg.ContextPolicy.MaxAsyncResultRunes)

			if len(s.messages) > prevLen {
				// 收到了新的异步结果，继续循环让 LLM 处理。
				slog.Info("loop: 收到异步 Agent 结果，继续循环",
					"新消息数", len(s.messages)-prevLen)
				continue
			}

			// 若还有待完成的异步 Agent，阻塞等待第一条结果后继续。
			if cfg.HasPendingAsync != nil && cfg.HasPendingAsync() {
				slog.Info("loop: 等待异步子 Agent 完成...")
				select {
				case <-ctx.Done():
					return Result{StopReason: StopAborted, TurnCount: s.turnCount,
						Err: ctx.Err(), Messages: s.messages}
				case msg := <-cfg.AsyncResults:
					s.messages = append(s.messages, truncateAsyncMessage(msg, cfg.ContextPolicy.MaxAsyncResultRunes))
					// 再次非阻塞 drain，收集同批完成的其他结果。
					s.messages = drainAsyncResults(s.messages, cfg.AsyncResults, cfg.ContextPolicy.MaxAsyncResultRunes)
					slog.Info("loop: 异步结果已注入，继续循环",
						"当前消息数", len(s.messages))
					continue
				}
			}

			result := Result{
				StopReason: StopCompleted,
				TurnCount:  s.turnCount,
				Answer:     turn.Content,
				Messages:   s.messages,
			}
			emit(events, Event{Kind: EventDone, Result: &result})
			return result
		}

		// A — Act：执行所有工具调用（并发/串行策略由 tools 包决定）。
		toolResults := act(ctx, cfg, turn.ToolCalls, events)

		// O — Observe：将工具结果转换为 role=tool 消息，注入上下文。
		observeMsgs := observe(toolResults, cfg, events)

		next := TransitionNextTurn
		allMsgs := append(s.messages, observeMsgs...)
		// 工具执行期间可能有异步 Agent 完成，非阻塞 drain 收集结果。
		allMsgs = drainAsyncResults(allMsgs, cfg.AsyncResults, cfg.ContextPolicy.MaxAsyncResultRunes)
		s = state{
			messages:   allMsgs,
			turnCount:  s.turnCount + 1,
			transition: &next,
		}

		slog.Info("loop: 本轮完成",
			"轮次", s.turnCount-1,
			"工具调用数", len(turn.ToolCalls),
		)
	}
}

// think 调用 LLM 并通过 SSE 流式接收完整的 [llm.AssistantTurn]。
// 对应 TAOR 的 T（Think）阶段。
func think(
	ctx context.Context,
	cfg Config,
	history []Message,
	events chan<- Event,
) (*llm.AssistantTurn, error) {
	slog.Info("loop: T — 调用模型", "model", cfg.Model, "历史长度", len(history))

	req := llm.ChatRequest{
		Model:     cfg.Model,
		Messages:  buildChatMessages(cfg.SystemPrompt, history),
		MaxTokens: cfg.MaxTokens,
	}
	if cfg.Registry != nil {
		req.Tools = cfg.Registry.LLMTools()
	}

	var finalTurn *llm.AssistantTurn
	for ev := range cfg.LLM.Stream(ctx, req) {
		if ev.Err != nil {
			return nil, fmt.Errorf("loop: 模型错误: %w", ev.Err)
		}
		if ev.TextDelta != "" {
			emit(events, Event{Kind: EventTextDelta, Delta: ev.TextDelta})
		}
		if ev.ThinkingDelta != "" {
			emit(events, Event{Kind: EventThinkingDelta, Delta: ev.ThinkingDelta})
		}
		if ev.Turn != nil {
			finalTurn = ev.Turn
		}
	}

	if finalTurn == nil {
		return nil, fmt.Errorf("loop: 模型流结束但未收到完整 turn")
	}

	slog.Info("loop: T — 完成",
		"finish_reason", finalTurn.FinishReason,
		"工具调用数", len(finalTurn.ToolCalls),
		"文本长度", len(finalTurn.Content),
	)
	return finalTurn, nil
}

// act 执行 toolCalls 中所有工具，返回 [tools.Result] 列表。
// 对应 TAOR 的 A（Act）阶段；并发/串行策略由 [tools.RunTools] 决定。
func act(
	ctx context.Context,
	cfg Config,
	toolCalls []llm.ToolCall,
	events chan<- Event,
) []tools.Result {
	if cfg.Registry == nil || len(toolCalls) == 0 {
		return nil
	}
	slog.Info("loop: A — 执行工具", "数量", len(toolCalls))

	results := tools.RunTools(ctx, toolCalls, cfg.Registry, cfg.CanUseTool)

	for _, r := range results {
		slog.Info("loop: A — 工具结果", "工具", r.ToolName, "错误", r.IsError)
		emit(events, Event{
			Kind:       EventToolResult,
			ToolName:   r.ToolName,
			ToolCallID: r.ToolCallID,
			ToolOutput: r.Output,
			IsError:    r.IsError,
		})
	}
	return results
}

// observe 将工具结果列表转换为 role=tool 消息并返回，供追加到对话历史。
// 对应 TAOR 的 O（Observe）阶段。
func observe(results []tools.Result, cfg Config, _ chan<- Event) []Message {
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
	slog.Info("loop: O — 观察完成", "结果数", len(msgs))
	return msgs
}

// truncateRunes 将 s 截断至最多 maxRunes 个 Unicode 标量值，必要时追加省略说明。
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

// report 调用 stopHook 并返回其结果。
// 返回非空字符串表示需要注入阻塞错误并重新进入循环；返回空字符串表示可以正常结束。
// 对应 TAOR 的 R（Report）阶段。
func report(history []Message, stopHook func([]Message) string) string {
	if stopHook == nil {
		return ""
	}
	return stopHook(history)
}

// buildChatMessages 将内部 [Message] 历史映射为 OpenAI 兼容的 [llm.ChatMessage] 切片。
// systemPrompt 非空时作为第一条 system 角色消息插入。
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
				Role:    "assistant",
				Content: m.Content,
				// [compat:deepseek] DeepSeek Reasoner 要求 reasoning_content 原样回传。
				// 标准 OpenAI 端点会忽略此字段，无副作用。
				ReasoningContent: m.Thinking,
			}
			if len(m.ToolCalls) > 0 {
				msg.ToolCalls = m.ToolCalls
				// 部分 API 要求携带 tool_calls 时 content 字段为空。
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

// emit 向 ch 发送事件；ch 为 nil 或 ch 已满时直接返回，不阻塞主循环。
func emit(ch chan<- Event, ev Event) {
	if ch == nil {
		return
	}
	select {
	case ch <- ev:
	default:
	}
}

// transitionStr 将跃迁原因格式化为可读字符串；t 为 nil 时返回 "initial"。
func transitionStr(t *TransitionReason) string {
	if t == nil {
		return "initial"
	}
	return string(*t)
}

// toolCallSummary 格式化单个工具调用为摘要字符串，用于调试日志。
func toolCallSummary(tc llm.ToolCall) string {
	args := tc.Function.Arguments
	return fmt.Sprintf("%s(%s)", tc.Function.Name, args)
}

// formatToolResults 格式化工具结果列表为多行摘要字符串，用于调试日志。
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

// marshalArgs 将 args map 序列化为紧凑 JSON 字符串，仅用于日志展示。
func marshalArgs(args map[string]any) string {
	b, _ := json.Marshal(args)
	return string(b)
}

// 以下空白赋值防止编译器将上述辅助函数标记为未使用。
var _ = toolCallSummary
var _ = formatToolResults
var _ = marshalArgs
