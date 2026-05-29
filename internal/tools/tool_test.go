package tools

import (
	"context"
	"strings"
	"testing"

	"matrix/internal/llm"
)

func TestRunTools_unknownBuiltinTool_delegateHint(t *testing.T) {
	reg := NewRegistry()
	results := RunTools(context.Background(), []llm.ToolCall{
		{
			ID: "call-1",
			Function: llm.ToolCallFunction{
				Name:      "glob",
				Arguments: `{"pattern":"**/*.md"}`,
			},
		},
	}, reg, nil, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results", len(results))
	}
	r := results[0]
	if !r.IsError {
		t.Fatal("expected error result")
	}
	if strings.Contains(r.Output, "unknown tool") {
		t.Fatalf("expected delegate hint, got: %s", r.Output)
	}
	if !strings.Contains(r.Output, "agent") || !strings.Contains(r.Output, "Worker") {
		t.Fatalf("unexpected output: %s", r.Output)
	}
}

func TestRunTools_unknownTool_plainMessage(t *testing.T) {
	reg := NewRegistry()
	results := RunTools(context.Background(), []llm.ToolCall{
		{
			ID: "call-2",
			Function: llm.ToolCallFunction{
				Name:      "not_a_real_tool",
				Arguments: `{}`,
			},
		},
	}, reg, nil, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results", len(results))
	}
	if results[0].Output != "unknown tool: not_a_real_tool" {
		t.Fatalf("got: %s", results[0].Output)
	}
}

func TestIsKnownBuiltinTool(t *testing.T) {
	if !IsKnownBuiltinTool("glob") {
		t.Fatal("glob should be a known builtin")
	}
	if IsKnownBuiltinTool("agent") {
		t.Fatal("agent is coordinator-only, not in DefaultRegistry")
	}
	if IsKnownBuiltinTool("not_a_real_tool") {
		t.Fatal("unexpected builtin")
	}
}
