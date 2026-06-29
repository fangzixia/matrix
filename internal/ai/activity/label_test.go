package activity

import "testing"

func TestTurnThinkingLabel(t *testing.T) {
	if got := TurnThinkingLabel(1, ""); got != "思考中…" {
		t.Fatalf("turn 1 = %q", got)
	}
	if got := TurnThinkingLabel(2, ""); got != "第 2 轮 · 思考中…" {
		t.Fatalf("turn 2 = %q", got)
	}
	if got := TurnThinkingLabel(2, "stop_hook_blocking"); got != "第 2 轮 · 校验未通过，正在重试…" {
		t.Fatalf("blocking = %q", got)
	}
}

func TestToolActivityLabel(t *testing.T) {
	if got := ToolActivityLabel("read_file", "started"); got != "正在调用 read_file" {
		t.Fatalf("started = %q", got)
	}
	if got := ToolActivityLabel("grep", "streaming"); got != "grep · 输出中…" {
		t.Fatalf("streaming = %q", got)
	}
}

func TestTurnWithToolsLabel(t *testing.T) {
	if got := TurnWithToolsLabel(2, 3); got != "第 2 轮 · 已调用 3 个工具" {
		t.Fatalf("with tools = %q", got)
	}
}
