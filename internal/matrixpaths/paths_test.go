package matrixpaths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceIDStable(t *testing.T) {
	a := WorkspaceID(`E:\AI\matrix`)
	b := WorkspaceID(`e:/ai/matrix`)
	if a == "" || a != b {
		t.Fatalf("id a=%q b=%q", a, b)
	}
}

func TestSessionsDirUsesAppData(t *testing.T) {
	appRoot := t.TempDir()
	SetDataRootForTest(appRoot)
	t.Cleanup(func() { SetDataRootForTest("") })

	ws := filepath.Join(t.TempDir(), "project")
	got := SessionsDir(ws)
	want := filepath.Join(appRoot, dirWorkspaces, WorkspaceID(ws), DirSessions)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if WorkspaceStoreJoin("", DirSessions) != "" {
		t.Fatal("empty workspace should yield empty path")
	}
}

func TestEnsureWorkspaceStore(t *testing.T) {
	appRoot := t.TempDir()
	SetDataRootForTest(appRoot)
	t.Cleanup(func() { SetDataRootForTest("") })

	ws := filepath.Join(t.TempDir(), "repo")
	if err := EnsureWorkspaceStore(ws); err != nil {
		t.Fatal(err)
	}

	storeRoot := filepath.Join(appRoot, dirWorkspaces, WorkspaceID(ws))
	for _, sub := range []string{DirSessions, DirChatTranscripts, DirSubagents, DirExports} {
		if info, err := os.Stat(filepath.Join(storeRoot, sub)); err != nil || !info.IsDir() {
			t.Fatalf("missing dir %s: %v", sub, err)
		}
	}

	meta, err := readMeta(ws)
	if err != nil || meta == nil || meta.WorkspaceID != WorkspaceID(ws) {
		t.Fatalf("meta: %+v err=%v", meta, err)
	}

	if err := EnsureWorkspaceStore(ws); err != nil {
		t.Fatal(err)
	}
}

func TestLogFile(t *testing.T) {
	appRoot := t.TempDir()
	SetDataRootForTest(appRoot)
	t.Cleanup(func() { SetDataRootForTest("") })

	got, err := LogFile()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(appRoot, DirLogs, LogFileName)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
