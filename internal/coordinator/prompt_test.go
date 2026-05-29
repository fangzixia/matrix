package coordinator

import (
	"strings"
	"testing"
)

func TestBuildParentSystemPrompt_excludesWorkerBasePrompt(t *testing.T) {
	p := BuildParentSystemPrompt([]string{"glob", "grep", "read_file"}, nil)
	if strings.Contains(p, "可以使用文件系统工具完成任务") {
		t.Fatal("parent prompt must not include worker-oriented BaseSystemPrompt")
	}
	if !strings.Contains(p, "不可直接调用") {
		t.Fatal("parent prompt should state worker tools are not callable by coordinator")
	}
	if strings.Contains(p, "Workers spawned via the agent tool have access to these tools") {
		t.Fatal("old worker context wording should be replaced")
	}
}

func TestBuildParentSystemPrompt_includesCoordinatorRole(t *testing.T) {
	p := BuildParentSystemPrompt(nil, nil)
	if !strings.Contains(p, "任务协调者") {
		t.Fatal("missing coordinator role")
	}
	if !strings.Contains(p, "禁止直接调用 glob") {
		t.Fatal("missing explicit ban on worker tool names")
	}
}

func TestBuildParentSystemPrompt_listsWorkerToolsAsUnavailable(t *testing.T) {
	p := BuildParentSystemPrompt([]string{"glob", "agent"}, []string{"playwright"})
	if !strings.Contains(p, "glob") || !strings.Contains(p, "仅 Worker 可用") {
		t.Fatal(p)
	}
	if !strings.Contains(p, "playwright") || !strings.Contains(p, "MCP") {
		t.Fatal("MCP servers should be marked worker-only", p)
	}
}
