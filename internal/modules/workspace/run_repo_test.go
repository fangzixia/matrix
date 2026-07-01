package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCopyDirTree(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	src := filepath.Join(base, "src")
	dst := filepath.Join(base, "dst")
	if err := os.MkdirAll(filepath.Join(src, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "pkg", "main.go"), []byte("package pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyDirTree(src, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "pkg", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package pkg\n" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestRunGitCloneCmd_timeout(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	cmd := exec.Command("sleep", "10")
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-Command", "Start-Sleep", "-Seconds", "10")
	}
	_, err := runGitCloneCmd(ctx, cmd)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Fatalf("unexpected error: %v", err)
	}
}
