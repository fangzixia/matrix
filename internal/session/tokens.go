// Package session 提供会话级上下文管理，分层对标 claude-code 的 compact / microCompact：
//
//   - [EstimateTokens]     — 粗略 token 估算（用于触发阈值，非精确计费）
//   - [ApplyMicroCompact]   — 微压缩：将较早的可压缩工具结果替换为占位文本
//   - [AppendCompactBoundary] — 注入 compact_boundary 风格的 system 锚点
//   - [PrepareHistory]     — 组合策略，供 [query.Config.PrepareHistory] 使用
//
// 依赖 [query.Message]；本包不反向引用 query 的执行逻辑，避免循环依赖。
package session

import (
	"matrix/internal/llm"
	"matrix/internal/query"
)

// EstimateTokens 对消息历史做保守的 token 粗算。
// 对标 claude-code 的 roughTokenCountEstimation：按字符估算并乘系数，
// 用于触发微压缩 / 告警，不应作为计费的精确值。
func EstimateTokens(msgs []query.Message) int {
	var chars int
	for _, m := range msgs {
		chars += len(m.Content)
		chars += len(m.Thinking)
		for _, tc := range m.ToolCalls {
			chars += len(tc.Function.Name) + len(tc.Function.Arguments)
		}
		if m.ToolName != "" {
			chars += len(m.ToolName)
		}
	}
	// 与常见经验法则一致：中文/代码混合下略偏保守
	return (chars + 3) / 4
}

// AssistantTurnTokens 估算单轮 AssistantTurn 的体量（用于日志）。
func AssistantTurnTokens(t *llm.AssistantTurn) int {
	if t == nil {
		return 0
	}
	chars := len(t.Content) + len(t.Thinking)
	for _, tc := range t.ToolCalls {
		chars += len(tc.Function.Name) + len(tc.Function.Arguments)
	}
	return (chars + 3) / 4
}
