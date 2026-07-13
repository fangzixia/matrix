package query

import (
	"context"
	"encoding/json"
	"fmt"
	"matrix/ai/llm"
	"strings"
	"unicode/utf8"
)

const (
	defaultContextSafetyMarginTokens = 2048
	defaultToolResultPlaceholder     = "[旧工具输出已压缩，如需完整内容请重新调用该工具]"
)

type contextPipelineStats struct {
	BeforeTokens  int
	AfterTokens   int
	Compacted     bool
	HardCompacted bool
	LLMCompacted  bool
}

// prepareHistoryForRequest 在 LLM 请求前对历史消息执行上下文治理流水线。
func prepareHistoryForRequest(ctx context.Context, cfg Config, msgs *[]Message) contextPipelineStats {
	stats := contextPipelineStats{}
	if msgs == nil {
		return stats
	}
	stats.BeforeTokens = estimateRequestTokens(cfg, *msgs)
	if applyMicroCompact(msgs, cfg.ContextPolicy, false) {
		stats.Compacted = true
	}
	if shouldProactiveAutoCompact(cfg, *msgs) {
		if compacted, ok := llmCompactHistory(ctx, cfg, *msgs, 0, "auto"); ok {
			*msgs = compacted
			stats.Compacted = true
			stats.LLMCompacted = true
		}
	}
	if enforceHardContextBudget(ctx, cfg, msgs) {
		stats.HardCompacted = true
		stats.Compacted = true
	}
	stats.AfterTokens = estimateRequestTokens(cfg, *msgs)
	if stats.Compacted {
		data := map[string]any{
			"before_tokens_est": stats.BeforeTokens,
			"after_tokens_est":  stats.AfterTokens,
			"hard_compacted":    stats.HardCompacted,
			"llm_compacted":     stats.LLMCompacted,
			"messages":          len(*msgs),
		}
		if cfg.Audit != nil {
			cfg.Audit.Emit("context.compact", 0, auditComponent(cfg), data)
		}
		agentLog(ctx, "query: 上下文流水线已压缩",
			"before_tokens_est", stats.BeforeTokens,
			"after_tokens_est", stats.AfterTokens,
			"hard_compacted", stats.HardCompacted,
			"llm_compacted", stats.LLMCompacted,
			"messages", len(*msgs),
		)
	}
	return stats
}

// applyMicroCompact 清理较早工具结果以释放上下文空间。
func applyMicroCompact(msgs *[]Message, p ContextPolicy, force bool) bool {
	if msgs == nil || len(*msgs) == 0 {
		return false
	}
	if !force {
		if p.MicroCompactThreshold <= 0 {
			return false
		}
		if estimateMessagesTokens(*msgs) < p.MicroCompactThreshold {
			return false
		}
	}
	keep := p.KeepRecentToolResults
	if keep < 1 {
		keep = 1
	}
	var idxs []int
	for i := range *msgs {
		if (*msgs)[i].Role == RoleTool {
			idxs = append(idxs, i)
		}
	}
	if len(idxs) <= keep {
		return false
	}
	placeholder := p.ClearedPlaceholder
	if placeholder == "" {
		placeholder = defaultToolResultPlaceholder
	}
	changed := false
	for _, idx := range idxs[:len(idxs)-keep] {
		if (*msgs)[idx].Content == placeholder {
			continue
		}
		(*msgs)[idx].Content = placeholder
		changed = true
	}
	return changed
}

// enforceHardContextBudget 在超出硬预算时强制截断历史。
func enforceHardContextBudget(ctx context.Context, cfg Config, msgs *[]Message) bool {
	limit := cfg.ContextPolicy.ContextLimitTokens
	if limit <= 0 || msgs == nil {
		return false
	}
	margin := cfg.ContextPolicy.ContextSafetyMarginTokens
	if margin <= 0 {
		margin = defaultContextSafetyMarginTokens
	}
	budget := limit - cfg.MaxTokens - margin
	if budget <= 0 {
		return false
	}
	if estimateRequestTokens(cfg, *msgs) <= budget {
		return false
	}
	applyMicroCompact(msgs, cfg.ContextPolicy, true)
	if estimateRequestTokens(cfg, *msgs) <= budget {
		return true
	}
	if compacted, ok := llmCompactHistory(ctx, cfg, *msgs, budget, "hard_budget"); ok {
		*msgs = compacted
		if estimateRequestTokens(cfg, *msgs) <= budget {
			return true
		}
	}
	*msgs = deterministicCompactToBudget(cfg, *msgs, budget)
	return true
}

// deterministicCompactToBudget 按预算确定性压缩历史消息。
func deterministicCompactToBudget(cfg Config, msgs []Message, budget int) []Message {
	summary := Message{
		Role:    RoleSystem,
		Content: buildDeterministicSummary(msgs),
	}
	if estimateRequestTokens(cfg, []Message{summary}) > budget {
		summary.Content = TruncateRunes(summary.Content, max(200, budget*2))
	}
	kept := make([]Message, 0, min(len(msgs), 16))
	for i := len(msgs) - 1; i >= 0; i-- {
		normalized := normalizeForCompactTail(msgs[i])
		candidate := append([]Message{summary}, append([]Message{normalized}, kept...)...)
		if estimateRequestTokens(cfg, candidate) > budget {
			continue
		}
		kept = append([]Message{normalized}, kept...)
	}
	if len(kept) == 0 && len(msgs) > 0 {
		last := normalizeForCompactTail(msgs[len(msgs)-1])
		last.Content = TruncateRunes(messageText(last), max(200, budget*2))
		kept = append(kept, last)
	}
	return append([]Message{summary}, kept...)
}

// buildDeterministicSummary 构建确定性摘要文本。
func buildDeterministicSummary(msgs []Message) string {
	var toolCount, userCount, assistantCount int
	var recent []string
	start := max(0, len(msgs)-8)
	for i, m := range msgs {
		switch m.Role {
		case RoleTool:
			toolCount++
		case RoleUser:
			userCount++
		case RoleAssistant:
			assistantCount++
		}
		if i >= start {
			recent = append(recent, fmt.Sprintf("- %s: %s", m.Role, TruncateRunes(messageText(m), 260)))
		}
	}
	return fmt.Sprintf(`%s
The earlier conversation was deterministically compacted before an LLM request to stay within the model context window.
Original history summary: %d messages, %d user messages, %d assistant messages, %d tool results.
Recent pre-compact messages:
%s`, compactBoundaryPrefix, len(msgs), userCount, assistantCount, toolCount, strings.Join(recent, "\n"))
}

// normalizeForCompactTail 规范化保留尾部消息用于压缩。
func normalizeForCompactTail(m Message) Message {
	switch m.Role {
	case RoleAssistant:
		if len(m.ToolCalls) > 0 {
			return Message{Role: RoleAssistant, Content: "Assistant requested tool calls: " + toolCallNames(m.ToolCalls)}
		}
		m.Thinking = ""
		return m
	case RoleTool:
		name := m.ToolName
		if name == "" {
			name = "tool"
		}
		return Message{Role: RoleUser, Content: fmt.Sprintf("[tool_result:%s] %s", name, TruncateRunes(m.Content, 1200))}
	default:
		m.Thinking = ""
		m.ToolCalls = nil
		return m
	}
}

// toolCallNames 提取消息中的工具调用名称列表。
func toolCallNames(calls []llm.ToolCall) string {
	names := make([]string, 0, len(calls))
	for _, tc := range calls {
		if tc.Function.Name != "" {
			names = append(names, tc.Function.Name)
		}
	}
	if len(names) == 0 {
		return fmt.Sprintf("%d calls", len(calls))
	}
	return strings.Join(names, ", ")
}

// estimateRequestTokens 估算单次 LLM 请求的 token 数。
func estimateRequestTokens(cfg Config, history []Message) int {
	req := llm.ChatRequest{
		Model:     cfg.Model,
		Messages:  buildChatMessages(cfg.SystemPrompt, history),
		MaxTokens: cfg.MaxTokens,
	}
	if cfg.Registry != nil {
		req.Tools = cfg.Registry.LLMTools()
	}
	b, _ := json.Marshal(req)
	return estimateCharsTokens(len(b))
}

// estimateMessagesTokens 估算消息列表总 token 数。
func estimateMessagesTokens(msgs []Message) int {
	var chars int
	for _, m := range msgs {
		chars += len(messageText(m))
		for _, tc := range m.ToolCalls {
			chars += len(tc.Function.Name) + len(tc.Function.Arguments)
		}
	}
	return estimateCharsTokens(chars)
}

// estimateCharsTokens 按字符数保守估算 token 数。
func estimateCharsTokens(chars int) int {
	if chars <= 0 {
		return 0
	}
	// 有意比 chars/4 更保守，为中文、代码、JSON 转义、消息包装及提供方模板预留余量。
	return (chars + 2) / 3
}

// messageText 提取单条消息用于估算的文本内容。
func messageText(m Message) string {
	var b strings.Builder
	if m.Content != "" {
		b.WriteString(m.Content)
	}
	if m.Thinking != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(m.Thinking)
	}
	return b.String()
}

// truncateAsyncMessage 按策略截断异步 Worker 结果消息。
func truncateAsyncMessage(m Message, maxRunes int) Message {
	if maxRunes <= 0 || utf8.RuneCountInString(m.Content) <= maxRunes {
		return m
	}
	m.Content = TruncateRunes(m.Content, maxRunes)
	return m
}

// min 返回两个整数中的较小值。
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max 返回两个整数中的较大值。
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
