package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"matrix/internal/matrixpaths"
)

func TestRedactString(t *testing.T) {
	in := "key=sk-abcdefghijklmnopqrstuvwxyz and Bearer secret123"
	out := RedactString(in)
	if strings.Contains(out, "sk-abc") {
		t.Fatalf("expected redacted sk key, got %q", out)
	}
	if strings.Contains(out, "secret123") {
		t.Fatalf("expected redacted bearer, got %q", out)
	}
}

func TestWriterEmitAndClose(t *testing.T) {
	matrixpaths.SetDataRootForTest(t.TempDir())
	t.Cleanup(func() { matrixpaths.SetDataRootForTest("") })

	dir := t.TempDir()
	w := NewWriter(dir, "sess-1")
	w.Emit("session.start", 0, "desktop", map[string]any{"model": "test-model"})
	w.Emit("turn.iteration", 1, "query", map[string]any{"message_count": 3})
	_ = w.Close(SessionMeta{StopReason: "completed", TurnCount: 1, DurationMs: 100})

	jsonl := filepath.Join(matrixpaths.SessionsDir(dir), "sess-1.jsonl")
	data, err := os.ReadFile(jsonl)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "session.start") {
		t.Fatalf("missing session.start: %s", data)
	}
	if !strings.Contains(string(data), "turn.iteration") {
		t.Fatalf("missing turn.iteration: %s", data)
	}

	metaPath := filepath.Join(matrixpaths.SessionsDir(dir), "sess-1.meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(metaData), "completed") {
		t.Fatalf("meta missing stop_reason: %s", metaData)
	}
}

func TestFormatForLLM(t *testing.T) {
	bundle := ExportBundle{
		Meta: SessionMeta{
			SessionID:   "abc",
			StartedAt:   "2026-05-21T09:00:00Z",
			StopReason:  "model_error",
			TaskPreview: "fix the bug",
			Error:       "timeout",
		},
		Events: []Event{
			{Ts: "2026-05-21T09:00:01Z", Event: "turn.llm_request", Turn: 1, Data: map[string]any{"model": "gpt"}},
		},
	}
	out := FormatForLLM(bundle)
	if !strings.Contains(out, "Session Diagnostic") {
		t.Fatal("missing title")
	}
	if !strings.Contains(out, "model_error") {
		t.Fatal("missing stop_reason")
	}
	if !strings.Contains(out, "turn.llm_request") {
		t.Fatal("missing timeline event")
	}
}

func TestListSessions(t *testing.T) {
	matrixpaths.SetDataRootForTest(t.TempDir())
	t.Cleanup(func() { matrixpaths.SetDataRootForTest("") })

	dir := t.TempDir()
	w := NewWriter(dir, "s-a")
	w.UpdateMeta(SessionMeta{Model: "m1"})
	_ = w.Close(SessionMeta{StopReason: "completed", TurnCount: 2})

	list, err := ListSessions(dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].SessionID != "s-a" {
		t.Fatalf("unexpected list: %+v", list)
	}
}
