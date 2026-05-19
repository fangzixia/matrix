package session

import (
	"testing"

	"matrix/internal/query"
)

func TestEstimateTokens(t *testing.T) {
	msgs := []query.Message{
		{Role: query.RoleUser, Content: "hello"},
		{Role: query.RoleAssistant, Content: "world", Thinking: "think"},
	}
	n := EstimateTokens(msgs)
	if n < 1 {
		t.Fatalf("预估 token 应 > 0，得 %d", n)
	}
}

func TestApplyMicroCompact(t *testing.T) {
	var msgs []query.Message
	for i := 0; i < 5; i++ {
		msgs = append(msgs, query.Message{
			Role: query.RoleTool, ToolName: "grep",
			Content: "long output " + string(rune('0'+i)),
		})
	}
	p := Policy{
		MicroCompactThreshold:  1,
		KeepRecentToolResults:  2,
		CompactableTools:       nil,
		ClearedPlaceholder:     "<cleared>",
		SkipIfContainsBoundary: false,
	}
	if !ApplyMicroCompact(&msgs, p) {
		t.Fatal("应发生微压缩")
	}
	phCount := 0
	fullCount := 0
	for _, m := range msgs {
		if m.Role != query.RoleTool {
			continue
		}
		if m.Content == "<cleared>" {
			phCount++
		} else {
			fullCount++
		}
	}
	if phCount != 3 || fullCount != 2 {
		t.Fatalf("期望 3 条占位 + 2 条全文，实际 ph=%d full=%d", phCount, fullCount)
	}
}

func TestApplyMicroCompact_SkipBoundary(t *testing.T) {
	msgs := []query.Message{
		{Role: query.RoleSystem, Content: "[compact_boundary]\n{}"},
		{Role: query.RoleTool, ToolName: "x", Content: "data"},
		{Role: query.RoleTool, ToolName: "x", Content: "data2"},
		{Role: query.RoleTool, ToolName: "x", Content: "data3"},
	}
	p := Policy{
		MicroCompactThreshold:  1,
		KeepRecentToolResults:  1,
		SkipIfContainsBoundary: true,
	}
	if ApplyMicroCompact(&msgs, p) {
		t.Fatal("存在 boundary 时应跳过微压缩")
	}
}
