package tasks

import (
	"strings"
	"testing"
)

func TestBuildSpecTask_includesPresetAndFile(t *testing.T) {
	s := BuildSpecTask("我的需求", ".spec/REQ-00001.md")
	if !strings.Contains(s, "创建需求") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "REQ-00001.md") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "我的需求") {
		t.Fatal(s)
	}
}

func TestBuildSpecTask_defaultWhenEmptyInput(t *testing.T) {
	s := BuildSpecTask("", "")
	if !strings.Contains(s, "编写需求文档") {
		t.Fatal(s)
	}
}
