package session

import (
	"strings"

	"matrix/internal/query"
)

// ApplyMicroCompact 按 [Policy] 原地压缩可压缩的 tool 消息（时间/local 路径，对标 microCompact 占位策略）。
// 返回是否发生了修改。
func ApplyMicroCompact(msgs *[]query.Message, p Policy) bool {
	if msgs == nil || len(*msgs) == 0 {
		return false
	}
	if p.MicroCompactThreshold <= 0 || p.KeepRecentToolResults <= 0 {
		return false
	}
	if EstimateTokens(*msgs) < p.MicroCompactThreshold {
		return false
	}
	if p.SkipIfContainsBoundary && hasCompactBoundary(*msgs) {
		return false
	}

	compactable := collectCompactableToolIndices(*msgs, p.CompactableTools)
	if len(compactable) <= p.KeepRecentToolResults {
		return false
	}
	cutOff := len(compactable) - p.KeepRecentToolResults
	ph := p.placeholder()
	changed := false
	for _, idx := range compactable[:cutOff] {
		m := &(*msgs)[idx]
		if m.Role != query.RoleTool || m.Content == ph {
			continue
		}
		m.Content = ph
		changed = true
	}
	return changed
}

func collectCompactableToolIndices(msgs []query.Message, allow map[string]struct{}) []int {
	var idxs []int
	for i := range msgs {
		if msgs[i].Role != query.RoleTool {
			continue
		}
		name := msgs[i].ToolName
		if allow != nil && len(allow) > 0 {
			if _, ok := allow[name]; !ok {
				continue
			}
		}
		idxs = append(idxs, i)
	}
	return idxs
}

func hasCompactBoundary(msgs []query.Message) bool {
	for _, m := range msgs {
		if m.Role == query.RoleSystem && strings.HasPrefix(m.Content, compactBoundaryMarker) {
			return true
		}
	}
	return false
}
