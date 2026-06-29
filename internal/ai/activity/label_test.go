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

func TestIsGenericTurnSummary(t *testing.T) {
	if !IsGenericTurnSummary("第 3 轮") {
		t.Fatal("expected generic")
	}
	if !IsGenericTurnSummary("第 1 轮（跃迁: next_turn）") {
		t.Fatal("expected跃迁 generic")
	}
	if IsGenericTurnSummary("读取 internal/foo.go") {
		t.Fatal("expected meaningful")
	}
}

func TestDeriveTurnSummaryFromTools(t *testing.T) {
	got := DeriveTurnSummary(1, []ToolSummaryInput{{
		Name:    "read_file",
		Preview: `{"target_path":"internal/coordinator/hub.go"}`,
	}}, "", "")
	if got != "读取 internal/coordinator/hub.go" {
		t.Fatalf("read = %q", got)
	}

	got = DeriveTurnSummary(2, []ToolSummaryInput{
		{Name: "grep", Preview: `{"pattern":"TurnSummary"}`},
		{Name: "read_file", Preview: `{"path":"label.go"}`},
	}, "", "")
	want := "grep TurnSummary · 读取 label.go"
	if got != want {
		t.Fatalf("multi = %q want %q", got, want)
	}
}

func TestDeriveTurnSummaryAgentDescription(t *testing.T) {
	got := DeriveTurnSummary(1, []ToolSummaryInput{{
		Name:    "agent",
		Preview: `{"description":"调研协调者模式实现"}`,
	}}, "", "")
	if got != "调研协调者模式实现" {
		t.Fatalf("agent = %q", got)
	}
}

func TestDeriveTurnSummaryFromLiveOutput(t *testing.T) {
	got := DeriveTurnSummary(1, []ToolSummaryInput{{
		Name:       "grep",
		LiveOutput: "grep \"coordinator\" @ internal …\n",
	}}, "", "")
	if got != `grep "coordinator" @ internal` {
		t.Fatalf("live = %q", got)
	}
}

func TestDeriveTurnSummaryMessageFallback(t *testing.T) {
	got := DeriveTurnSummary(1, nil, "已完成调研，结论如下。", "")
	if got != "已完成调研，结论如下。" {
		t.Fatalf("message = %q", got)
	}
}

func TestDeriveTurnSummaryThinkingFallback(t *testing.T) {
	got := DeriveTurnSummary(1, nil, "", "正在分析代码结构…")
	if got != "思考中…" {
		t.Fatalf("thinking = %q", got)
	}
}

func TestDeriveTurnSummaryTurnFallback(t *testing.T) {
	got := DeriveTurnSummary(3, nil, "", "")
	if got != "第 3 轮" {
		t.Fatalf("fallback = %q", got)
	}
}
