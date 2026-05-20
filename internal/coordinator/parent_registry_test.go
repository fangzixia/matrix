package coordinator

import (
	"testing"

	"matrix/internal/agent"
	"matrix/internal/tools"
)

func TestNewParentRegistry_onlyCoordinatorTools(t *testing.T) {
	parent := NewParentRegistry(Config{AgentRegistry: agent.NewRegistry()})
	names := parent.Names()
	if len(names) != 3 {
		t.Fatalf("parent tool count = %d, want 3: %v", len(names), names)
	}
	for _, want := range []string{"agent", "send_message", "task_stop"} {
		if parent.Get(want) == nil {
			t.Errorf("parent missing tool %q", want)
		}
	}
	for _, name := range names {
		if !IsParentAllowedTool(name) {
			t.Errorf("parent tool %q not in allowlist", name)
		}
	}
}

func TestCloneWorkerRegistry_excludesCoordinatorTools(t *testing.T) {
	base := tools.DefaultRegistry()
	worker := CloneWorkerRegistry(base)
	for name := range ParentToolNames {
		if worker.Get(name) != nil {
			t.Errorf("worker must not have coordinator tool %q", name)
		}
	}
	if worker.Get("read_file") == nil {
		t.Error("worker missing read_file")
	}
	if worker.Get("bash") == nil {
		t.Error("worker missing bash")
	}
}

func TestParentWorkerRegistryDisjoint(t *testing.T) {
	base := tools.DefaultRegistry()
	worker := CloneWorkerRegistry(base)
	parent := NewParentRegistry(Config{AgentRegistry: agent.NewRegistry()})

	for _, name := range parent.Names() {
		if worker.Get(name) != nil {
			t.Errorf("tool %q must not appear in both parent and worker registries", name)
		}
	}
}

func TestIsParentAllowedTool_prActivityMCP(t *testing.T) {
	if !IsParentAllowedTool("mcp__github__subscribe_pr_activity") {
		t.Error("expected PR subscribe MCP suffix to be parent-allowed")
	}
	if IsParentAllowedTool("mcp__github__create_issue") {
		t.Error("generic MCP should not be parent-allowed by suffix rule")
	}
	if IsParentAllowedTool("read_file") {
		t.Error("read_file must not be parent-allowed")
	}
}
