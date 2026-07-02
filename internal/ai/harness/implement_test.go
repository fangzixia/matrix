package harness

import (
	"strings"
	"testing"
)

func TestImplementWorkflow_withEval(t *testing.T) {
	planAbs := `E:\workspace\docs\plans\PLAN-foo.md`
	evalAbs := `E:\workspace\docs\evaluations\EVAL-PLAN-foo-20260701.md`
	prompt := BuildImplementTask("修复 AC-2", planAbs, evalAbs)
	if !strings.Contains(prompt, planAbs) {
		t.Fatalf("expected plan path in prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, evalAbs) {
		t.Fatalf("expected eval path in prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, "评测报告:") {
		t.Fatalf("expected eval label in prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, "未通过项与修复建议") {
		t.Fatalf("expected eval guidance in preset:\n%s", prompt)
	}
}

func TestImplementWorkflow_withoutEval(t *testing.T) {
	planAbs := `E:\workspace\docs\plans\PLAN-foo.md`
	prompt := BuildImplementTask("实现功能", planAbs, "")
	if !strings.Contains(prompt, planAbs) {
		t.Fatalf("expected plan path in prompt:\n%s", prompt)
	}
	if strings.Contains(prompt, "评测报告:") {
		t.Fatalf("unexpected eval section in prompt:\n%s", prompt)
	}
}
