package coordinator

import (
	"matrix/ai/query"
	"matrix/ai/util"
)

// QueryConfigOverrides 覆盖 QueryConfigFromCoordinator 的默认字段。
type QueryConfigOverrides struct {
	SystemPrompt    string
	Registry        *util.Registry
	AsyncResults    <-chan query.Message
	HasPendingAsync func() bool
	InitialMessages []query.Message
	LogPrefix       string
	MaxTurns        int
}

// QueryConfigFromCoordinator 从 Coordinator Config 构建 query.Config 公共字段。
func QueryConfigFromCoordinator(cfg Config, o QueryConfigOverrides) query.Config {
	maxTurns := o.MaxTurns
	if maxTurns == 0 {
		maxTurns = cfg.MaxTurns
	}
	if maxTurns == 0 {
		maxTurns = 200
	}
	return query.Config{
		LLM:                cfg.LLM,
		Model:              cfg.Model,
		SystemPrompt:       o.SystemPrompt,
		Registry:           o.Registry,
		MaxTurns:           maxTurns,
		MaxTokens:          cfg.MaxTokens,
		ContextPolicy:      cfg.ContextPolicy,
		MaxToolResultRunes: cfg.MaxToolResultRunes,
		CanUseTool:         cfg.CanUseTool,
		AsyncResults:       o.AsyncResults,
		HasPendingAsync:    o.HasPendingAsync,
		LogPrefix:          o.LogPrefix,
		ThreadID:           cfg.ThreadID,
		RunID:              cfg.RunID,
		SessionID:          cfg.SessionID,
		Audit:              cfg.Audit,
		InitialMessages:    o.InitialMessages,
	}
}
