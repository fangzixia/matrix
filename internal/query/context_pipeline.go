package query

import (
	"context"
	"encoding/json"
	"fmt"
	"matrix/internal/logger"
	"strings"
	"unicode/utf8"

	"matrix/internal/llm"
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
		logger.InfoCtx(ctx, "query: context pipeline compacted",
			"before_tokens_est", stats.BeforeTokens,
			"after_tokens_est", stats.AfterTokens,
			"hard_compacted", stats.HardCompacted,
			"llm_compacted", stats.LLMCompacted,
			"messages", len(*msgs),
		)
	}
	return stats
}

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

func deterministicCompactToBudget(cfg Config, msgs []Message, budget int) []Message {
	summary := Message{
		Role:    RoleSystem,
		Content: buildDeterministicSummary(msgs),
	}
	if estimateRequestTokens(cfg, []Message{summary}) > budget {
		summary.Content = truncateRunes(summary.Content, max(200, budget*2))
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
		last.Content = truncateRunes(messageText(last), max(200, budget*2))
		kept = append(kept, last)
	}
	return append([]Message{summary}, kept...)
}

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
			recent = append(recent, fmt.Sprintf("- %s: %s", m.Role, truncateRunes(messageText(m), 260)))
		}
	}
	return fmt.Sprintf(`[compact_boundary]
The earlier conversation was deterministically compacted before an LLM request to stay within the model context window.
Original history summary: %d messages, %d user messages, %d assistant messages, %d tool results.
Recent pre-compact messages:
%s`, len(msgs), userCount, assistantCount, toolCount, strings.Join(recent, "\n"))
}

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
		return Message{Role: RoleUser, Content: fmt.Sprintf("[tool_result:%s] %s", name, truncateRunes(m.Content, 1200))}
	default:
		m.Thinking = ""
		m.ToolCalls = nil
		return m
	}
}

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

func estimateCharsTokens(chars int) int {
	if chars <= 0 {
		return 0
	}
	// Deliberately more conservative than chars/4 to leave room for mixed
	// Chinese, code, JSON escaping, message wrappers, and provider templates.
	return (chars + 2) / 3
}

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

func truncateAsyncMessage(m Message, maxRunes int) Message {
	if maxRunes <= 0 || utf8.RuneCountInString(m.Content) <= maxRunes {
		return m
	}
	m.Content = truncateRunes(m.Content, maxRunes)
	return m
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
