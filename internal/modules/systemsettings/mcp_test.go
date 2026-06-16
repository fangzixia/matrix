package systemsettings

import (
	"encoding/json"
	"testing"
)

func TestSaveValidateAfterMergeWorkerDefaults(t *testing.T) {
	cfg := DefaultTestConfig()
	merged := Settings{
		Models: []ModelProfileSettings{{
			ID: "1", Name: "t", Model: "m", MaxTokens: 100, Enabled: true, Default: true,
		}},
		Worker: WorkerSettings{
			MaxAttempts:  cfg.Worker.MaxAttempts,
			Concurrency:  cfg.Worker.Concurrency,
			PollInterval: cfg.Worker.PollInterval.String(),
			Enabled:      true,
		},
		Security: SecuritySettings{ShellTimeout: "60s"},
		Git:      GitSettings{CloneTimeout: "300s"},
		MCPServers: map[string]MCPServerSettings{
			"x": {Command: "npx", Args: []string{"-y", "pkg"}},
		},
	}
	if err := validate(merged); err != nil {
		t.Fatalf("validate after merge: %v", err)
	}
}

func TestMCPServerEnvRoundTrip(t *testing.T) {
	in := map[string]MCPServerSettings{
		"mysql": {
			Command: "npx",
			Args:    []string{"-y", "mysql-query-mcp-server@latest"},
			Env:     map[string]string{"LOCAL_DB_HOST": "127.0.0.1"},
		},
	}
	out := toMCPServers(in)
	if out["mysql"].Env["LOCAL_DB_HOST"] != "127.0.0.1" {
		t.Fatalf("env not preserved: %+v", out["mysql"].Env)
	}
	back := fromMCPServers(out)
	raw, _ := json.Marshal(back["mysql"])
	if !json.Valid(raw) {
		t.Fatal("invalid json")
	}
}
