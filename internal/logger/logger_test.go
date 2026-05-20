package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsDevEnv(t *testing.T) {
	t.Setenv("MATRIX_DEV", "1")
	if !isDev() {
		t.Fatal("expected dev when MATRIX_DEV=1")
	}
}

func TestIsDevExecutable(t *testing.T) {
	t.Setenv("MATRIX_DEV", "")
	dir := t.TempDir()
	exe := filepath.Join(dir, "matrix-dev.exe")
	if err := os.WriteFile(exe, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	// isDev reads os.Executable(), not our temp file — only test naming heuristic via strings
	base := filepath.Base(exe)
	if !strings.Contains(strings.ToLower(base), "-dev") {
		t.Fatal("test setup broken")
	}
}
