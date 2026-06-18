package systemsettings

import (
	"encoding/json"
	"testing"
)

func TestSaveValidateAfterMergeWorkerDefaults(t *testing.T) {
	cfg := DefaultTestConfig()
	if err := validateAI(AISettings{
		Models: []ModelProfileSettings{{
			ID: "1", Name: "t", Model: "m", MaxTokens: 100, Enabled: true, Default: true,
		}},
		Security: SecuritySettings{ShellTimeout: "60s"},
	}); err != nil {
		t.Fatalf("validate ai: %v", err)
	}
	if err := validateWorker(WorkerSettings{
		MaxAttempts:  cfg.Worker.MaxAttempts,
		Concurrency:  cfg.Worker.Concurrency,
		PollInterval: cfg.Worker.PollInterval.String(),
		Enabled:      true,
	}); err != nil {
		t.Fatalf("validate worker: %v", err)
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
