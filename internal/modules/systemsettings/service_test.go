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
	merged := in
	s.apply(merged, false)
	if s.cfg.AI.ActiveModel().Model != "m" {
		t.Fatalf("expected active model m, got %q", s.cfg.AI.ActiveModel().Model)
	}
	if s.cfg.MCP.Servers["demo"].Command != "node" {
		t.Fatalf("cfg not updated: %+v", s.cfg.MCP.Servers)
	}
}

func TestMigrateLegacyModel(t *testing.T) {
	st := Settings{
		Model: ModelSettings{
			BaseURL: "https://x", Model: "legacy", MaxTokens: 100, APIKey: "k", APIKeySet: true,
		},
	}
	migrateLegacyModel(&st)
	if len(st.Models) != 1 || st.Models[0].Model != "legacy" {
		t.Fatalf("unexpected %+v", st.Models)
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

func TestValidate(t *testing.T) {
	if err := validate(Settings{
		Models:   []ModelProfileSettings{{ID: "1", Name: "a", Model: "m", MaxTokens: 100, Enabled: true}},
		Worker:   WorkerSettings{MaxAttempts: 3, Concurrency: 2, PollInterval: "2s"},
		Security: SecuritySettings{ShellTimeout: "60s"},
		Git:      GitSettings{CloneTimeout: "300s"},
	}); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
}

func TestNormalizeModelSettingsDefault(t *testing.T) {
	st := Settings{
		Models: []ModelProfileSettings{
			{ID: "1", Name: "a", Model: "m1", Enabled: true, Default: true, MaxTokens: 100},
			{ID: "2", Name: "b", Model: "m2", Enabled: true, Default: true, MaxTokens: 100},
		},
	}
	normalizeModelSettings(&st)
	count := 0
	for _, m := range st.Models {
		if m.Default {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected one default, got %d", count)
	}
}
