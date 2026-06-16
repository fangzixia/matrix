package harness

import (
	"strings"
	"testing"
	"time"
)

func TestBuildSpecTask_coordinatorDelegationWording(t *testing.T) {
	s := BuildSpecTask("需求", "")
	if strings.Contains(s, "使用 read/grep") {
		t.Fatal("spec steps should not instruct coordinator to call read/grep directly")
	}
	if !strings.Contains(s, "通过 agent") {
		t.Fatal("spec steps should delegate via agent")
	}
	if !strings.Contains(s, "Coordinator 执行说明") {
		t.Fatal("missing coordinator execution note")
	}
}

func TestBuildSpecTask_create_usesSPECTimestamp(t *testing.T) {
	now := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	s := newSpecWorkflowAt("我的需求", "", now).Prompt()
	if !strings.Contains(s, ".matrix/SPEC-20260102150405.md") {
		t.Fatalf("missing generated spec path: %s", s)
	}
	if !strings.Contains(s, "操作模式: create") {
		t.Fatal("missing create mode")
	}
	if !strings.Contains(s, "创建需求") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "验收标准") || !strings.Contains(s, "风险") {
		t.Fatal("missing preset sections")
	}
	if !strings.Contains(s, "我的需求") {
		t.Fatal(s)
	}
}

func TestBuildSpecTask_update_usesGivenPath(t *testing.T) {
	now := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	path := ".matrix/SPEC-00001.md"
	s := newSpecWorkflowAt("更新需求", path, now).Prompt()
	if !strings.Contains(s, path) {
		t.Fatalf("missing target path: %s", s)
	}
	if !strings.Contains(s, "操作模式: update") {
		t.Fatal("missing update mode")
	}
	if strings.Contains(s, "SPEC-20260102150405") {
		t.Fatal("should not generate new spec path in update mode")
	}
}

func TestBuildSpecTask_includesWorkflowMeta(t *testing.T) {
	s := BuildSpecTask("我的需求", ".matrix/SPEC-20260101120000.md")
	if !strings.Contains(s, "kind: spec") || !strings.Contains(s, "expected_artifacts") {
		t.Fatal(s)
	}
}

func TestBuildSpecTask_defaultWhenEmptyInput(t *testing.T) {
	s := BuildSpecTask("", "")
	if !strings.Contains(s, "编写需求文档") {
		t.Fatal(s)
	}
}

func TestResolveSpecTarget(t *testing.T) {
	now := time.Date(2026, 5, 28, 14, 30, 22, 0, time.UTC)
	target, mode := resolveSpecTarget("", now)
	if mode != "create" || target != ".matrix/SPEC-20260528143022.md" {
		t.Fatalf("got %q %q", target, mode)
	}
	target, mode = resolveSpecTarget("  .matrix/SPEC-old.md  ", now)
	if mode != "update" || target != ".matrix/SPEC-old.md" {
		t.Fatalf("got %q %q", target, mode)
	}
}
