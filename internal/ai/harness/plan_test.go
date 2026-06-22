package harness

import (
	"strings"
	"testing"
	"time"
)

func TestBuildPlanTask_coordinatorDelegationWording(t *testing.T) {
	s := BuildPlanTask("计划", "")
	if strings.Contains(s, "使用 read/grep") {
		t.Fatal("plan steps should not instruct coordinator to call read/grep directly")
	}
	if !strings.Contains(s, "通过 agent") {
		t.Fatal("plan steps should delegate via agent")
	}
	if !strings.Contains(s, "Coordinator 执行说明") {
		t.Fatal("missing coordinator execution note")
	}
}

func TestBuildPlanTask_create_usesPLANTimestamp(t *testing.T) {
	now := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	s := newPlanWorkflowAt("我的计划", "", now).Prompt()
	if !strings.Contains(s, ".matrix/PLAN-20260102150405.md") {
		t.Fatalf("missing generated plan path: %s", s)
	}
	if !strings.Contains(s, "操作模式: create") {
		t.Fatal("missing create mode")
	}
	if !strings.Contains(s, "创建计划") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "验收标准") || !strings.Contains(s, "风险") {
		t.Fatal("missing preset sections")
	}
	if !strings.Contains(s, "我的计划") {
		t.Fatal(s)
	}
}

func TestBuildPlanTask_update_usesGivenPath(t *testing.T) {
	now := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	path := ".matrix/PLAN-00001.md"
	s := newPlanWorkflowAt("更新计划", path, now).Prompt()
	if !strings.Contains(s, path) {
		t.Fatalf("missing target path: %s", s)
	}
	if !strings.Contains(s, "操作模式: update") {
		t.Fatal("missing update mode")
	}
	if strings.Contains(s, "PLAN-20260102150405") {
		t.Fatal("should not generate new plan path in update mode")
	}
}

func TestBuildPlanTask_includesWorkflowMeta(t *testing.T) {
	s := BuildPlanTask("我的计划", ".matrix/PLAN-20260101120000.md")
	if !strings.Contains(s, "kind: plan") || !strings.Contains(s, "expected_artifacts") {
		t.Fatal(s)
	}
}

func TestBuildPlanTask_defaultWhenEmptyInput(t *testing.T) {
	s := BuildPlanTask("", "")
	if !strings.Contains(s, "编写计划文档") {
		t.Fatal(s)
	}
}

func TestResolvePlanTarget(t *testing.T) {
	now := time.Date(2026, 5, 28, 14, 30, 22, 0, time.UTC)
	target, mode := resolvePlanTarget("", now)
	if mode != "create" || target != ".matrix/PLAN-20260528143022.md" {
		t.Fatalf("got %q %q", target, mode)
	}
	target, mode = resolvePlanTarget("  .matrix/PLAN-old.md  ", now)
	if mode != "update" || target != ".matrix/PLAN-old.md" {
		t.Fatalf("got %q %q", target, mode)
	}
}
