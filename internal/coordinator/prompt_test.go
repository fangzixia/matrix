package coordinator

import (
	"strings"
	"testing"
)

func TestBuildParentSystemPrompt(t *testing.T) {
	p := BuildParentSystemPrompt([]string{"read_file", "bash"}, []string{"playwright"})
	if !strings.Contains(p, "有用的 AI 助手") {
		t.Fatal("missing base system prompt")
	}
	if !strings.Contains(p, "协调") {
		t.Fatal("missing coordinator prompt")
	}
	if !strings.Contains(p, "read_file") || !strings.Contains(p, "bash") {
		t.Fatal("missing worker tools")
	}
	if !strings.Contains(p, "playwright") {
		t.Fatal("missing mcp servers")
	}
	if strings.Contains(p, "场景预设") {
		t.Fatal("should not include scenario overlay")
	}
}
