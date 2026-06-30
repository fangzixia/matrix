package storage

import (
	"matrix/internal/platform/config"
	"os"
	"path/filepath"
	"testing"
)

func TestRunRepoDir(t *testing.T) {
	p := Paths{WorkspacesDir: filepath.Join("data", "workspaces")}
	got := RunRepoDir(p, "my-project", "run-123")
	want := filepath.Join("data", "workspaces", "my-project", "runs", "run-123", "repo")
	if got != want {
		t.Fatalf("RunRepoDir got %q, want %q", got, want)
	}
	sandbox := RunSandboxDir(p, "my-project", "run-123")
	wantSandbox := filepath.Join("data", "workspaces", "my-project", "runs", "run-123")
	if sandbox != wantSandbox {
		t.Fatalf("RunSandboxDir got %q, want %q", sandbox, wantSandbox)
	}
}

func TestProjectMatrixRunDir(t *testing.T) {
	p := Paths{WorkspacesDir: filepath.Join("data", "workspaces")}
	got := ProjectMatrixRunDir(p, "my-project", "run-123")
	want := filepath.Join("data", "workspaces", "my-project", ".matrix", "runs", "run-123")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveDataDir(t *testing.T) {
	root := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	cfg := func(dataDir string) *config.Config {
		return &config.Config{
			Storage: config.StorageConfig{DataDir: dataDir},
			Logging: config.LoggingConfig{Dir: "./logs"},
		}
	}

	t.Run("relative", func(t *testing.T) {
		p, err := Resolve(cfg("./data"))
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(root, "data")
		assertSamePath(t, p.DataDir, want)
		assertSamePath(t, p.WorkspacesDir, filepath.Join(want, "workspaces"))
	})

	t.Run("absolute", func(t *testing.T) {
		absData := filepath.Join(root, "var", "matrix-data")
		p, err := Resolve(cfg(absData))
		if err != nil {
			t.Fatal(err)
		}
		assertSamePath(t, p.DataDir, absData)
		assertSamePath(t, p.WorkspacesDir, filepath.Join(absData, "workspaces"))
	})
}

func assertSamePath(t *testing.T, got, want string) {
	t.Helper()
	gotAbs, err := filepath.Abs(got)
	if err != nil {
		t.Fatal(err)
	}
	wantAbs, err := filepath.Abs(want)
	if err != nil {
		t.Fatal(err)
	}
	if gotAbs != wantAbs {
		t.Fatalf("got %q, want %q", gotAbs, wantAbs)
	}
}
