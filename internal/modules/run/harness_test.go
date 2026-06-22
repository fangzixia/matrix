package run

import (
	"strings"
	"testing"
)

func TestBuildHarnessMessages_plan(t *testing.T) {
	msgs := BuildHarnessMessages("plan", "我的计划", "", "")
	if len(msgs) != 1 || msgs[0].Role != "user" {
		t.Fatal(msgs)
	}
	if !strings.Contains(msgs[0].Content, "创建计划") {
		t.Fatal(msgs[0].Content)
	}
}

func TestBuildHarnessMessages_implement(t *testing.T) {
	msgs := BuildHarnessMessages("implement", "实现功能", ".matrix/PLAN-1.md", "")
	if !strings.Contains(msgs[0].Content, "编码实现") {
		t.Fatal(msgs[0].Content)
	}
}

func TestBuildHarnessMessages_chat(t *testing.T) {
	msgs := BuildHarnessMessages("chat", "hello", "", "")
	if msgs[0].Content != "hello" {
		t.Fatal(msgs[0].Content)
	}
}

func TestNormalizeStageKind(t *testing.T) {
	if normalizeStageKind("spec") != "plan" {
		t.Fatal("legacy spec should map to plan")
	}
	if normalizeStageKind("implement") != "implement" {
		t.Fatal()
	}
}
