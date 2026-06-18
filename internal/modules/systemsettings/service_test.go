package systemsettings

import (
	"testing"

	"matrix/internal/platform/config"
)

func TestSaveAndGetMCPServers(t *testing.T) {
	cfg := DefaultTestConfig()
	s := &Service{cfg: cfg}
	in := Settings{
		Models: []ModelProfileSettings{{
			ID: "m1", Name: "test", BaseURL: "https://api.example.com", APIKey: "k",
			Model: "m", MaxTokens: 100, Enabled: true, Default: true,
		}},
		MCPServers: map[string]MCPServerSettings{
			"demo": {Command: "node", Args: []string{"server.js"}},
		},
		Worker:   WorkerSettings{MaxAttempts: 3, Concurrency: 2, PollInterval: "2s"},
		Security: SecuritySettings{ShellTimeout: "60s"},
		Git:      GitSettings{CloneTimeout: "300s"},
	}
	s.apply(in, false)
	if s.cfg.AI.ActiveModel().Model != "m" {
		t.Fatalf("expected active model m, got %q", s.cfg.AI.ActiveModel().Model)
	}
	if s.cfg.MCP.Servers["demo"].Command != "node" {
		t.Fatalf("cfg not updated: %+v", s.cfg.MCP.Servers)
	}
}

func TestMergeModelAPIKeys(t *testing.T) {
	out := []ModelProfileSettings{{ID: "a", APIKey: ""}}
	existing := []ModelProfileSettings{{ID: "a", APIKey: "secret"}}
	mergeModelAPIKeys(&out, existing)
	if out[0].APIKey != "secret" {
		t.Fatal("expected preserved key")
	}
}

func TestToGitConfigMultipleAccesses(t *testing.T) {
	g := toGitConfig(GitSettings{
		CloneTimeout: "300s",
		Accesses: []GitAccess{
			{ID: "1", Name: "GitHub", Host: "github.com", SSHKeyPath: "/keys/gh"},
			{ID: "2", Name: "GitLab", Host: "gitlab.com", SSHKeyPath: "/keys/gl"},
		},
	})
	if len(g.Accesses) != 2 {
		t.Fatalf("expected 2 accesses, got %d", len(g.Accesses))
	}
}

func TestFromGitConfig(t *testing.T) {
	g := fromGitConfig(config.GitConfig{
		CloneTimeout: config.Default().Git.CloneTimeout,
		Accesses: []config.GitAccessConfig{
			{ID: "a", Name: "默认", Host: "*", SSHKeyPath: "/k"},
		},
	})
	if len(g.Accesses) != 1 || g.Accesses[0].SSHKeyPath != "/k" {
		t.Fatalf("unexpected %+v", g)
	}
}

func TestValidateWorker(t *testing.T) {
	if err := validateWorker(WorkerSettings{MaxAttempts: 3, Concurrency: 2, PollInterval: "2s"}); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}

func TestValidateAI(t *testing.T) {
	if err := validateAI(AISettings{
		Models:   []ModelProfileSettings{{ID: "1", Name: "a", Model: "m", MaxTokens: 100, Enabled: true}},
		Security: SecuritySettings{ShellTimeout: "60s"},
	}); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}

func TestNormalizeModelProfilesDefault(t *testing.T) {
	models := []ModelProfileSettings{
		{ID: "1", Name: "a", Model: "m1", Enabled: true, Default: true, MaxTokens: 100},
		{ID: "2", Name: "b", Model: "m2", Enabled: true, Default: true, MaxTokens: 100},
	}
	normalizeModelProfiles(&models)
	count := 0
	for _, m := range models {
		if m.Default {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected one default, got %d", count)
	}
}
