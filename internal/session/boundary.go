package session

import (
	"encoding/json"
	"fmt"

	"matrix/internal/query"
)

// compactBoundaryMarker 为注入到 system 消息开头的魔数，便于识别与跳过重复微压缩。
const compactBoundaryMarker = "[compact_boundary]"

// CompactMeta 记录一次压缩边界的元数据（对标 SystemCompactBoundaryMessage 的精简版）。
type CompactMeta struct {
	// Kind 为 "manual" | "auto" | "micro_only"。
	Kind string `json:"kind"`
	// PreCompactTokens 为压缩前的 [EstimateTokens] 粗算值。
	PreCompactTokens int `json:"pre_compact_tokens_est"`
	// Note 为可选说明。
	Note string `json:"note,omitempty"`
}

// AppendCompactBoundary 向消息列表追加一条 system 锚点，便于后续 resume / 调试识别。
// 实际「全量摘要」可在上层调用 LLM 后替换 msgs 再调用本函数。
func AppendCompactBoundary(msgs *[]query.Message, meta CompactMeta) error {
	if msgs == nil {
		return fmt.Errorf("session: messages 指针为空")
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	line := fmt.Sprintf("%s\n%s", compactBoundaryMarker, string(b))
	*msgs = append(*msgs, query.Message{
		Role:    query.RoleSystem,
		Content: line,
	})
	return nil
}
