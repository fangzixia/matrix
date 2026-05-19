package query

import (
	"strings"
	"testing"
)

func TestPrepareHistoryForRequest_MicroCompact(t *testing.T) {
	msgs := []Message{
		{Role: RoleTool, Content: strings.Repeat("old ", 200), ToolName: "read"},
		{Role: RoleTool, Content: "recent", ToolName: "read"},
	}
	cfg := Config{
		ContextPolicy: ContextPolicy{
			MicroCompactThreshold: 1,
			KeepRecentToolResults: 1,
		},
	}

	prepareHistoryForRequest(cfg, &msgs)

	if !strings.Contains(msgs[0].Content, "已压缩") {
		t.Fatalf("expected old tool result to be compacted, got %q", msgs[0].Content)
	}
	if msgs[1].Content != "recent" {
		t.Fatalf("expected recent tool result to be preserved, got %q", msgs[1].Content)
	}
}

func TestPrepareHistoryForRequest_HardBudget(t *testing.T) {
	msgs := []Message{
		{Role: RoleUser, Content: strings.Repeat("alpha ", 2000)},
		{Role: RoleAssistant, Content: strings.Repeat("beta ", 2000)},
		{Role: RoleUser, Content: "latest question"},
	}
	cfg := Config{
		ContextPolicy: ContextPolicy{
			ContextLimitTokens:        500,
			ContextSafetyMarginTokens: 50,
		},
	}

	prepareHistoryForRequest(cfg, &msgs)

	if len(msgs) == 0 || msgs[0].Role != RoleSystem {
		t.Fatalf("expected compact boundary system message, got %+v", msgs)
	}
	if !strings.Contains(msgs[0].Content, "[compact_boundary]") {
		t.Fatalf("expected compact boundary marker, got %q", msgs[0].Content)
	}
}
