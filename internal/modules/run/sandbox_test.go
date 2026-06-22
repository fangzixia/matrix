package run

import (
	"testing"

	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
)

func TestShouldUseSharedLock(t *testing.T) {
	cfg := config.Default()
	s := &Service{cfg: cfg}

	m := &models.Run{SandboxPath: "/tmp/wt", RunBranch: "matrix/run-abc"}
	if s.shouldUseSharedLock(m) {
		t.Fatal("worktree run should not use shared lock")
	}

	m2 := &models.Run{}
	if !s.shouldUseSharedLock(m2) {
		t.Fatal("worktree mode without sandbox should use shared lock until prepared")
	}

	cfg.Run.SandboxMode = config.SandboxModeShared
	s2 := &Service{cfg: cfg}
	if !s2.shouldUseSharedLock(&models.Run{}) {
		t.Fatal("shared mode should always lock")
	}
}

func TestMergeStatusAfterRun(t *testing.T) {
	cfg := config.Default()
	s := &Service{cfg: cfg}
	m := &models.Run{SandboxPath: "/wt", RunBranch: "matrix/run-x"}
	if s.mergeStatusAfterRun(m, "succeeded") != "pending" {
		t.Fatal("expected pending merge status")
	}
	if s.mergeStatusAfterRun(m, "failed") != "" {
		t.Fatal("failed run should not set merge status")
	}
}
