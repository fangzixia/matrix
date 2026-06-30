package workspace

import (
	"os"
	"path/filepath"
	"testing"
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
