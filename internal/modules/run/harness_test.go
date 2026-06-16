package run

import (
	"strings"
	"testing"
)

func TestBuildHarnessMessages_spec(t *testing.T) {
	msgs := BuildHarnessMessages("spec", "我的需求", "", "")
	if len(msgs) != 1 || msgs[0].Role != "user" {
		t.Fatal("expected single user message")
	}
	if !strings.Contains(msgs[0].Content, "创建需求") {
		t.Fatalf("missing spec preset: %s", msgs[0].Content)
	}
}

func TestBuildHarnessMessages_implement(t *testing.T) {
	msgs := BuildHarnessMessages("implement", "实现功能", ".matrix/SPEC-1.md", "")
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
