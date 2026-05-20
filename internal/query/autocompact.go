package query

import (
	"context"
	"fmt"
	"matrix/internal/logger"
	"strings"

	"matrix/internal/llm"
)

const (
	defaultAutoCompactMaxTokens = 4096
	defaultKeepRecentMessages   = 8
	autoCompactSystemPrompt     = `You are a conversation summarizer for an AI coding assistant.
Summarize the transcript below so the assistant can continue with full context.
Preserve: user goals, decisions, file paths, errors, pending work, and important tool outcomes.
Use the same language as the conversation. Output plain text only, no markdown fences.`
)

// summarizeHistory 调用模型对历史消息做全量摘要。
func summarizeHistory(ctx context.Context, cfg Config, msgs []Message) (string, error) {
	if cfg.LLM == nil {
		return "", fmt.Errorf("autocompact: LLM 客户端未配置")
	}
	transcript := formatTranscriptForSummary(msgs)
	if strings.TrimSpace(transcript) == "" {
		return "", fmt.Errorf("autocompact: 无可摘要内容")
	}

	maxOut := defaultAutoCompactMaxTokens
	if cfg.MaxTokens > 0 && cfg.MaxTokens < maxOut {
		maxOut = cfg.MaxTokens
	}

	summary, err := cfg.LLM.Complete(ctx, llm.ChatRequest{
		Model: cfg.Model,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: autoCompactSystemPrompt},
			{Role: "user", Content: transcript},
		},
		MaxTokens: maxOut,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(summary), nil
}

func formatTranscriptForSummary(msgs []Message) string {
	var b strings.Builder
	for i, m := range msgs {
		if i > 0 {
			b.WriteByte('\n')
		}
		switch m.Role {
		case RoleAssistant:
			b.WriteString("[assistant]")
			if m.Thinking != "" {
				b.WriteString(" (thinking)\n")
				b.WriteString(truncateRunes(m.Thinking, 2000))
				b.WriteByte('\n')
			}
			if m.Content != "" {
				b.WriteString(m.Content)
			}
			if len(m.ToolCalls) > 0 {
				if m.Content != "" {
					b.WriteByte('\n')
				}
				b.WriteString("tool_calls: ")
				b.WriteString(toolCallNames(m.ToolCalls))
			}
		case RoleTool:
			name := m.ToolName
			if name == "" {
				name = "tool"
			}
			b.WriteString("[tool:")
			b.WriteString(name)
			b.WriteString("] ")
			b.WriteString(truncateRunes(m.Content, 4000))
		default:
			b.WriteString("[")
			b.WriteString(string(m.Role))
			b.WriteString("] ")
			b.WriteString(truncateRunes(messageText(m), 4000))
		}
	}
	return b.String()
}

// llmCompactHistory 用模型摘要替换较早历史，保留尾部消息，并注入 compact_boundary。
func llmCompactHistory(ctx context.Context, cfg Config, msgs []Message, budget int, kind string) ([]Message, bool) {
	if len(msgs) == 0 || cfg.LLM == nil {
		return msgs, false
	}
	keep := cfg.ContextPolicy.KeepRecentMessages
	if keep < 1 {
		keep = defaultKeepRecentMessages
	}
	if len(msgs) <= keep+1 {
		return msgs, false
	}

	head := msgs[:len(msgs)-keep]
	tail := msgs[len(msgs)-keep:]

	summary, err := summarizeHistory(ctx, cfg, head)
	if err != nil {
		logger.Warnf("query: LLM autoCompact 失败，将回退确定性压缩: %v", err)
		return msgs, false
	}

	preTokens := estimateMessagesTokens(msgs)
	boundary := buildLLMCompactSummary(summary, len(head), len(tail), preTokens, kind)
	out := append([]Message{{Role: RoleSystem, Content: boundary}}, tail...)

	if budget > 0 && estimateRequestTokens(cfg, out) > budget {
		// 摘要仍超长：截断 boundary 文本
		out[0].Content = truncateRunes(out[0].Content, max(400, budget*3))
		if estimateRequestTokens(cfg, out) > budget {
			return msgs, false
		}
	}

	logger.Info("query: LLM autoCompact 完成",
		"kind", kind,
		"messages_before", len(msgs),
		"messages_after", len(out),
		"pre_tokens_est", preTokens,
		"post_tokens_est", estimateRequestTokens(cfg, out),
	)
	return out, true
}

func buildLLMCompactSummary(summary string, headCount, tailCount, preTokens int, kind string) string {
	return fmt.Sprintf(`[compact_boundary]
The earlier conversation was summarized by the model before the next request (kind=%s).
Pre-compact estimate: ~%d tokens across %d messages; %d recent messages kept verbatim.
Summary:
%s`, kind, preTokens, headCount, tailCount, summary)
}

func hasCompactBoundary(msgs []Message) bool {
	for _, m := range msgs {
		if m.Role == RoleSystem && strings.HasPrefix(m.Content, "[compact_boundary]") {
			return true
		}
	}
	return false
}

func shouldProactiveAutoCompact(cfg Config, msgs []Message) bool {
	th := cfg.ContextPolicy.AutoCompactThreshold
	if th <= 0 || cfg.LLM == nil || len(msgs) < 4 {
		return false
	}
	if hasCompactBoundary(msgs) {
		return false
	}
	return estimateRequestTokens(cfg, msgs) >= th
}
