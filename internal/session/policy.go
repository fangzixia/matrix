package session

// Policy 定义上下文治理参数，对标 claude-code autoCompact + microCompact 的客户端侧子集。
type Policy struct {
	// MicroCompactThreshold 为估算 token 阈值；0 表示不启用微压缩。
	// 当 [EstimateTokens](messages) >= MicroCompactThreshold 时在每轮 Think 前尝试微压缩。
	MicroCompactThreshold int

	// KeepRecentToolResults 微压缩时保留最近若干条「可压缩」的 tool 消息全文，其余替换为占位符。
	// 须 >= 1 才会实际清除更早记录；0 表示禁用微压缩清除（仍可配合截断）。
	KeepRecentToolResults int

	// CompactableTools 给出允许被微压缩替换的工具名；nil 或空表示所有 RoleTool 均可压缩。
	CompactableTools map[string]struct{}

	// ClearedPlaceholder 替换旧工具输出时使用的占位字符串。
	// 为空时使用默认中文占位。
	ClearedPlaceholder string

	// SkipIfContainsBoundary 为 true 时：若历史中已有 compact_boundary，则不再做微压缩，
	// 避免在全量摘要后的短历史上重复清空。
	SkipIfContainsBoundary bool
}

// DefaultPolicy 返回可用的默认策略（偏保守：不自动压缩，仅保留字段默认）。
func DefaultPolicy() Policy {
	return Policy{
		MicroCompactThreshold:  0,
		KeepRecentToolResults:  3,
		ClearedPlaceholder:     "[旧工具输出已压缩，若需完整内容请重新调用该工具]",
		SkipIfContainsBoundary: true,
	}
}

func (p Policy) placeholder() string {
	if p.ClearedPlaceholder != "" {
		return p.ClearedPlaceholder
	}
	return "[旧工具输出已压缩，若需完整内容请重新调用该工具]"
}
