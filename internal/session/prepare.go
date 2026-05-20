package session

import (
	"matrix/internal/logger"

	"matrix/internal/query"
)

// PrepareHistory 返回适配 [query.Config.PrepareHistory] 的闭包，按 policy 在每轮 Think 前执行微压缩。
// 若 needLog 为 true，在发生压缩时打 Info 日志。
func PrepareHistory(policy Policy, needLog bool) func(*[]query.Message) {
	return func(msgs *[]query.Message) {
		if msgs == nil || len(*msgs) == 0 {
			return
		}
		before := EstimateTokens(*msgs)
		if policy.MicroCompactThreshold <= 0 || before < policy.MicroCompactThreshold {
			return
		}
		if ApplyMicroCompact(msgs, policy) && needLog {
			after := EstimateTokens(*msgs)
			logger.Info("session: 已执行微压缩",
				"估算token_前", before,
				"估算token_后", after,
				"保留最近工具条数", policy.KeepRecentToolResults,
			)
		}
	}
}
