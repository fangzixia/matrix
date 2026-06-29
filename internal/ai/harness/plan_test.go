package harness

import (
	"strings"
	"testing"
	"time"
)

func TestPlanWorkflowCreateWithResolvedAbsPath(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 17, 26, 0, time.UTC)
	abs := `E:\workspace\docs\plans\PLAN-20260629121726.md`
	w := newPlanWorkflowAt("把协调者模式增加开关", "", abs, now)
	prompt := w.Prompt()
	if !strings.Contains(prompt, "操作模式: create") {
		t.Fatalf("expected create mode, got prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, abs) {
		t.Fatalf("expected absolute target path in prompt, got:\n%s", prompt)
	}
}

func TestPlanWorkflowUpdateWithSelectedPath(t *testing.T) {
	selected := "docs/plans/PLAN-existing.md"
	abs := `E:\workspace\docs\plans\PLAN-existing.md`
	w := newPlanWorkflowAt("更新计划", selected, abs, time.Now())
	prompt := w.Prompt()
	if !strings.Contains(prompt, "操作模式: update") {
		t.Fatalf("expected update mode, got prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, abs) {
		t.Fatalf("expected absolute target path in prompt, got:\n%s", prompt)
	}
}
