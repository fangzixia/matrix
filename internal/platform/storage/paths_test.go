package storage

import (
	"path/filepath"
	"testing"
)

func TestProjectMatrixRunDir(t *testing.T) {
	p := Paths{WorkspacesDir: filepath.Join("data", "workspaces")}
	got := ProjectMatrixRunDir(p, "my-project", "run-123")
	want := filepath.Join("data", "workspaces", "my-project", ".matrix", "runs", "run-123")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
