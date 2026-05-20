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
	for _, section := range []string{
		"5. 先综合，再委派",
		"6. 任务阶段工作流",
		"7. send_message 续接 vs agent 新建",
		"8. 编写 Worker Prompt",
	} {
		if !strings.Contains(p, section) {
			t.Fatalf("missing coordinator section %q", section)
		}
	}
}

func TestCoordinatorSystemPrompt_synthesisAndWorkflow(t *testing.T) {
	if !strings.Contains(CoordinatorSystemPrompt, "根据你的结果") {
		t.Fatal("missing lazy-delegation anti-pattern")
	}
	if !strings.Contains(CoordinatorSystemPrompt, "调研 (Research)") {
		t.Fatal("missing research phase")
	}
	if !strings.Contains(CoordinatorSystemPrompt, "send_message") {
		t.Fatal("missing continue guidance")
	}
	if !strings.Contains(CoordinatorSystemPrompt, "自包含") {
		t.Fatal("missing self-contained prompt guidance")
	}
}

func TestWorkerSystemPrompt_modes(t *testing.T) {
	for _, phrase := range []string{"只调研", "验证", "完成标准"} {
		if !strings.Contains(WorkerSystemPrompt, phrase) {
			t.Fatalf("WorkerSystemPrompt missing %q", phrase)
		}
	}
}
