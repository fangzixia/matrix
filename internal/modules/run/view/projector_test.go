package view

import (
	"context"
	"matrix/internal/ai/activity"
	"matrix/internal/ai/stream"
	"testing"
)

func msgProgress(turn int, summary string) stream.Message {
	return stream.TurnProgress("run-1", turn, "next_turn", summary)
}

func msgTextDelta(text string) stream.Message {
	return stream.TextDelta("run-1", text, 0)
}

func msgThinkingDelta(thinking string) stream.Message {
	return stream.ThinkingDelta("run-1", thinking, 0)
}

func msgAssistant(text, thinking string) stream.Message {
	return stream.Assistant("run-1", text, thinking, nil, "end_turn")
}

func msgToolStarted(toolUseID, name string) stream.Message {
	return stream.ToolStarted("run-1", toolUseID, name, `{"path":"."}`)
}

func msgToolFinished(toolUseID, name, status string) stream.Message {
	return stream.ToolFinished("run-1", toolUseID, name, status, 100, "done")
}

func TestProjectorTurnProgress(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	envs := p.Apply(msgProgress(1, "第 1 轮"))
	if len(p.state.Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(p.state.Turns))
	}
	if p.state.Turns[0].Turn != 1 {
		t.Fatalf("turn num = %d", p.state.Turns[0].Turn)
	}
	_ = envs
}

func TestProjectorTurnSummaryFromTools(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.Apply(msgProgress(1, "第 1 轮"))
	p.Apply(stream.ToolStarted("run-1", "tool-1", "read_file", `{"target_path":"internal/foo.go"}`))
	if activity.IsGenericTurnSummary(p.state.Turns[0].Summary) {
		t.Fatalf("expected derived summary after tool start, got %q", p.state.Turns[0].Summary)
	}
	if p.state.Turns[0].Summary != "读取 internal/foo.go" {
		t.Fatalf("summary = %q", p.state.Turns[0].Summary)
	}
}

func TestProjectorTextDelta(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.Apply(stream.MessageStart("run-1"))
	envs := p.Apply(msgTextDelta("hello"))
	var found bool
	for _, e := range envs {
		if e.Type == EventTEXTMessageContent {
			found = true
			pl := e.Payload.(TextDeltaPayload)
			if pl.Delta != "hello" {
				t.Fatalf("delta = %q", pl.Delta)
			}
		}
	}
	if !found {
		t.Fatal("expected TEXT_MESSAGE_CONTENT")
	}
	if p.state.Turns[0].Message != "hello" {
		t.Fatalf("message = %q", p.state.Turns[0].Message)
	}
}

func TestProjectorThinkingDelta(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.Apply(stream.MessageStart("run-1"))
	envs := p.Apply(msgThinkingDelta("think"))
	found := false
	for _, e := range envs {
		if e.Type == EventREASONINGMessageContent {
			found = true
		}
	}
	if !found {
		t.Fatal("expected REASONING_MESSAGE_CONTENT")
	}
}

func TestProjectorAssistant(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.ensureCoordinatorTurn(1, "")
	p.Apply(msgAssistant("final answer", "thought"))
	if p.state.Turns[0].Message != "final answer" {
		t.Fatalf("message = %q", p.state.Turns[0].Message)
	}
	if p.state.Turns[0].Thinking != "thought" {
		t.Fatalf("thinking = %q", p.state.Turns[0].Thinking)
	}
}

func TestProjectorToolLifecycle(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.ensureCoordinatorTurn(1, "")
	start := p.Apply(msgToolStarted("tool-1", "read_file"))
	hasStart := false
	for _, e := range start {
		if e.Type == EventTOOLCallStart {
			hasStart = true
		}
	}
	if !hasStart {
		t.Fatal("expected TOOL_CALL_START")
	}
	done := p.Apply(msgToolFinished("tool-1", "read_file", "completed"))
	hasResult := false
	for _, e := range done {
		if e.Type == EventTOOLCallResult {
			hasResult = true
		}
	}
	if !hasResult {
		t.Fatal("expected TOOL_CALL_RESULT")
	}
	if len(p.state.Turns[0].Tools) != 1 {
		t.Fatalf("tools = %d", len(p.state.Turns[0].Tools))
	}
	if p.state.Turns[0].Tools[0].Status != "success" {
		t.Fatalf("status = %s", p.state.Turns[0].Tools[0].Status)
	}
	if p.state.Turns[0].Tools[0].LogURL == "" {
		t.Fatal("expected tool log URL in snapshot")
	}
}

func TestProjectorToolOutputDelta(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.ensureCoordinatorTurn(1, "")
	p.Apply(msgToolStarted("tool-1", "shell"))
	p.Apply(stream.ToolOutputDelta("run-1", "tool-1", "shell", "line1\n", 6))
	if p.state.Turns[0].Tools[0].LiveOutput != "line1\n" {
		t.Fatalf("liveOutput = %q", p.state.Turns[0].Tools[0].LiveOutput)
	}
}

func TestProjectorReconcilesProvisionalToolID(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.ensureCoordinatorTurn(1, "")
	p.Apply(stream.ToolInputStreaming("run-1", "pending-0", "write_file", "{"))
	p.Apply(msgToolStarted("call_real", "write_file"))
	p.Apply(msgToolFinished("call_real", "write_file", "completed"))

	tools := p.state.Turns[0].Tools
	if len(tools) != 1 {
		t.Fatalf("want 1 tool after reconcile, got %d: %+v", len(tools), tools)
	}
	if tools[0].ToolCallID != "call_real" {
		t.Fatalf("toolCallId = %q", tools[0].ToolCallID)
	}
	if tools[0].Status != "success" {
		t.Fatalf("status = %q", tools[0].Status)
	}
}

func TestProjectorCompletesStreamedToolAfterTurnAdvances(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.ensureCoordinatorTurn(1, "")
	p.Apply(msgToolStarted("parent-agent", "agent"))
	p.parse.workerParentToolID["agent-w"] = "parent-agent"

	tag := func(msg stream.Message) stream.Message {
		return stream.WithAgent(msg, "agent-w", "", "parent-agent")
	}

	p.Apply(tag(stream.TurnProgress("run-1", 1, "next_turn", "")))
	p.Apply(tag(msgToolStarted("call_grep", "grep")))
	p.Apply(tag(stream.ToolOutputDelta("run-1", "call_grep", "grep", "match line\n", 12)))

	// activeTurn 已切到第 2 轮，但完成事件应回写第 1 轮中的 grep
	p.Apply(tag(stream.TurnProgress("run-1", 2, "next_turn", "")))
	p.Apply(tag(msgToolFinished("call_grep", "grep", "completed")))

	agentTool := p.state.Turns[0].Tools[0]
	if len(agentTool.WorkerTurns) < 1 {
		t.Fatal("expected worker turn 1")
	}
	grep := agentTool.WorkerTurns[0].Tools[0]
	if grep.Status != "success" {
		t.Fatalf("status = %q", grep.Status)
	}
	if grep.OutputStreaming {
		t.Fatal("outputStreaming should be false after completed")
	}
	if grep.LiveOutput == "" {
		t.Fatal("expected streamed output preserved")
	}
}

func TestProjectorWorkerToolNotOnCoordinatorTurn(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.ensureCoordinatorTurn(1, "")
	p.Apply(msgToolStarted("parent-agent", "agent"))
	p.Apply(msgToolFinished("parent-agent", "agent", "completed"))
	p.parse.workerParentToolID["agent-worker"] = "parent-agent"

	workerStarted := stream.WithAgent(
		stream.ToolStarted("run-1", "call_w1", "list_dir", `{"path":"."}`),
		"agent-worker", "", "parent-agent",
	)
	p.Apply(workerStarted)
	p.Apply(stream.WithAgent(
		stream.ToolFinished("run-1", "call_w1", "list_dir", "completed", 10, "ok"),
		"agent-worker", "", "parent-agent",
	))

	if len(p.state.Turns[0].Tools) != 1 {
		t.Fatalf("coordinator tools = %d", len(p.state.Turns[0].Tools))
	}
	agentTool := p.state.Turns[0].Tools[0]
	if len(agentTool.WorkerTurns) != 1 {
		t.Fatalf("worker turns = %d", len(agentTool.WorkerTurns))
	}
	if len(agentTool.WorkerTurns[0].Tools) != 1 {
		t.Fatalf("worker tools = %d", len(agentTool.WorkerTurns[0].Tools))
	}
	if agentTool.WorkerTurns[0].Tools[0].Status != "success" {
		t.Fatalf("worker tool status = %q", agentTool.WorkerTurns[0].Tools[0].Status)
	}
}

func TestProjectorWorkerToolBeforeParentAgentProjected(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.ensureCoordinatorTurn(1, "")
	p.parse.workerParentToolID["agent-worker"] = "parent-agent"
	p.parse.parentToolCoordinatorTurn["parent-agent"] = 1

	workerStarted := stream.WithAgent(
		stream.ToolStarted("run-1", "call_w1", "grep", `{"pattern":"foo"}`),
		"agent-worker", "", "parent-agent",
	)
	p.Apply(workerStarted)
	p.Apply(stream.WithAgent(
		stream.ToolFinished("run-1", "call_w1", "grep", "completed", 10, "matches"),
		"agent-worker", "", "parent-agent",
	))

	if len(p.state.Turns[0].Tools) != 1 {
		t.Fatalf("expected placeholder agent tool, got %d tools", len(p.state.Turns[0].Tools))
	}
	agentTool := p.state.Turns[0].Tools[0]
	if agentTool.ToolCallID != "parent-agent" {
		t.Fatalf("agent tool id = %q", agentTool.ToolCallID)
	}
	if len(agentTool.WorkerTurns) != 1 || len(agentTool.WorkerTurns[0].Tools) != 1 {
		t.Fatalf("worker tools missing: turns=%d tools=%d",
			len(agentTool.WorkerTurns), len(agentTool.WorkerTurns[0].Tools))
	}
}

func TestProjectorResult(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.Apply(stream.ResultSuccessMsg("run-1", "output text", "end_turn", 3, 0))
	if p.state.Result == nil || p.state.Result.Output != "output text" {
		t.Fatal("expected result output")
	}
	if ExtractReplyText(p.state) != "output text" {
		t.Fatalf("reply = %q", ExtractReplyText(p.state))
	}
}

func TestProjectorResultError(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.Apply(stream.ResultErrorMsg("run-1", "error", "loop: 模型错误: bad key", 1, 0))
	if p.state.Result == nil || !p.state.Result.IsError {
		t.Fatal("expected error result")
	}
	if p.state.Error == "" {
		t.Fatal("expected formatted error")
	}
}

func TestMapToolStatus(t *testing.T) {
	if mapToolStatus("completed") != "success" {
		t.Fatal()
	}
	if mapToolStatus("failed") != "error" {
		t.Fatal()
	}
	if mapToolStatus("started") != "loading" {
		t.Fatal()
	}
}

func TestFormatUserRunErrorAPIKey(t *testing.T) {
	msg := FormatUserRunError("authentication failed: invalid api key")
	if msg == "" || msg == "authentication failed: invalid api key" {
		t.Fatalf("expected translated error, got %q", msg)
	}
}

func TestExtractReplyTextFromTurns(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.ensureCoordinatorTurn(1, "")
	p.state.Turns[0].Message = "hello world"
	if ExtractReplyText(p.state) != "hello world" {
		t.Fatalf("got %q", ExtractReplyText(p.state))
	}
}

func TestHubSeqMonotonic(t *testing.T) {
	s := NewStore(nil)
	e1 := s.withSeq("r1", Envelope{Type: EventRUNStarted})
	e2 := s.withSeq("r1", Envelope{Type: EventTEXTMessageContent})
	if e1.Seq != 1 || e2.Seq != 2 {
		t.Fatalf("seq %d %d", e1.Seq, e2.Seq)
	}
}

func TestStoreApplyAdvancesSnapshotSeq(t *testing.T) {
	s := NewStore(nil)
	ctx := context.Background()
	if err := s.BeginRun(ctx, "run-1", "proj-1", "chat", "chat"); err != nil {
		t.Fatal(err)
	}
	if err := s.apply(ctx, "run-1", "proj-1", msgProgress(1, "第 1 轮")); err != nil {
		t.Fatal(err)
	}
	first, err := s.Snapshot(ctx, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || first.Seq != 2 {
		t.Fatalf("expected seq 2 after first apply, got %+v", first)
	}
	if err := s.apply(ctx, "run-1", "proj-1", msgTextDelta("hello")); err != nil {
		t.Fatal(err)
	}
	second, err := s.Snapshot(ctx, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if second == nil || second.Seq != 3 {
		t.Fatalf("expected seq 3 after second apply, got %+v", second)
	}
}

func TestAllowedInChat(t *testing.T) {
	if !AllowedInChat(EventTEXTMessageContent) {
		t.Fatal()
	}
	if !AllowedInChat(EventACTIVITYSnapshot) {
		t.Fatal("activity snapshot should be allowed in chat")
	}
	if !AllowedInChat(EventSTATESnapshot) {
		t.Fatal("state snapshot should be allowed in chat")
	}
	if AllowedInChat(EventTOOLCallStart) {
		t.Fatal()
	}
}

func TestProjectorMessageStop(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.Apply(stream.MessageStart("run-1"))
	envs := p.Apply(stream.MessageStop("run-1"))
	hasEnd := false
	for _, e := range envs {
		if e.Type == EventTEXTMessageEnd {
			hasEnd = true
		}
	}
	if !hasEnd {
		t.Fatal("expected TEXT_MESSAGE_END")
	}
}

func TestProjectorMultipleTurns(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.Apply(msgProgress(1, "round 1"))
	p.Apply(msgProgress(2, "round 2"))
	if len(p.state.Turns) != 2 {
		t.Fatalf("turns = %d", len(p.state.Turns))
	}
}

func TestCloneState(t *testing.T) {
	p := NewProjector("run-1", "proj-1")
	p.ensureCoordinatorTurn(1, "s")
	c := cloneState(p.state)
	c.Turns[0].Message = "changed"
	if p.state.Turns[0].Message == "changed" {
		t.Fatal("clone should be independent for nested slice content when reassigned")
	}
}
