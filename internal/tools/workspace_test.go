package tools

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequireToolPath(t *testing.T) {
	if err := RequireToolPath(`C:\proj\src\foo`); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"", ".", "  ", " . "} {
		if err := RequireToolPath(p); !errors.Is(err, ErrToolPathRequired) {
			t.Fatalf("path %q: got %v", p, err)
		}
	}
}

func TestResolveWorkspacePath_absoluteOnly(t *testing.T) {
	dir := t.TempDir()
	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(absDir, "src")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveWorkspacePath(sub)
	if err != nil {
		t.Fatal(err)
	}
	if got != sub {
		t.Fatalf("got %q want %q", got, sub)
	}

	if _, err := ResolveWorkspacePath("src"); !errors.Is(err, ErrToolPathNotAbsolute) {
		t.Fatalf("relative path: %v", err)
	}
}

func TestResolveAndValidateToolPath(t *testing.T) {
	dir := t.TempDir()
	absRoot, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	SetWorkspaceRoot(absRoot)
	t.Cleanup(func() { SetWorkspaceRoot("") })

	if _, err := ResolveAndValidateToolPath(""); !errors.Is(err, ErrToolPathRequired) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := ResolveAndValidateToolPath("rel/path"); !errors.Is(err, ErrToolPathNotAbsolute) {
		t.Fatalf("relative: %v", err)
	}

	sub := filepath.Join(absRoot, "src")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveAndValidateToolPath(sub)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != sub {
		t.Fatalf("got %q want %q", resolved, sub)
	}
}

func TestFormatWorkerUserMessage(t *testing.T) {
	dir := t.TempDir()
	absDir, _ := filepath.Abs(dir)
	SetWorkspaceRoot(absDir)
	t.Cleanup(func() { SetWorkspaceRoot("") })

	msg := FormatWorkerUserMessage("do work")
	if want := "工作区: " + absDir + "\n\ndo work"; msg != want {
		t.Fatalf("got %q want %q", msg, want)
	}
}

func TestListDirRequiresPath(t *testing.T) {
	SetWorkspaceRoot(t.TempDir())
	t.Cleanup(func() { SetWorkspaceRoot("") })

	_, err := ListDir.Execute(t.Context(), map[string]any{})
	if !errors.Is(err, ErrToolPathRequired) {
		t.Fatalf("expected ErrToolPathRequired, got %v", err)
	}
}

func TestListDirAbsolutePath(t *testing.T) {
	ws := t.TempDir()
	absWS, err := filepath.Abs(ws)
	if err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(absWS, "charge-web")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	javaFile := filepath.Join(sub, "App.java")
	if err := os.WriteFile(javaFile, []byte("class App {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	SetWorkspaceRoot(absWS)
	t.Cleanup(func() { SetWorkspaceRoot("") })

	out, err := ListDir.Execute(t.Context(), map[string]any{"path": sub})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, javaFile) {
		t.Fatalf("list_dir: got\n%s", out)
	}
}
