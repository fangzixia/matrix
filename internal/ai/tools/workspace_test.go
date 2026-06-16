package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testCtx(t *testing.T, root string) context.Context {
	t.Helper()
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	return WithSandbox(t.Context(), abs)
}

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
	ctx := testCtx(t, dir)

	if _, err := ResolveAndValidateToolPath(ctx, ""); !errors.Is(err, ErrToolPathRequired) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := ResolveAndValidateToolPath(ctx, "rel/path"); !errors.Is(err, ErrToolPathNotAbsolute) {
		t.Fatalf("relative: %v", err)
	}

	absRoot := SandboxFrom(ctx)
	sub := filepath.Join(absRoot, "src")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveAndValidateToolPath(ctx, sub)
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

	msg := FormatWorkerUserMessage(absDir, "do work")
	if want := "沙箱目录: " + absDir + "\n\ndo work"; msg != want {
		t.Fatalf("got %q want %q", msg, want)
	}
}

func TestListDirRequiresPath(t *testing.T) {
	ctx := testCtx(t, t.TempDir())

	_, err := ListDir.Execute(ctx, map[string]any{})
	if !errors.Is(err, ErrToolPathRequired) {
		t.Fatalf("expected ErrToolPathRequired, got %v", err)
	}
}

func TestListDirAbsolutePath(t *testing.T) {
	ws := t.TempDir()
	ctx := testCtx(t, ws)
	absWS := SandboxFrom(ctx)
	sub := filepath.Join(absWS, "charge-web")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	javaFile := filepath.Join(sub, "App.java")
	if err := os.WriteFile(javaFile, []byte("class App {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := ListDir.Execute(ctx, map[string]any{"path": sub})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, javaFile) {
		t.Fatalf("list_dir: got\n%s", out)
	}
}

func TestSandboxFrom_requiresWithSandbox(t *testing.T) {
	if SandboxFrom(context.Background()) != "" {
		t.Fatal("expected empty sandbox without WithSandbox")
	}
}
