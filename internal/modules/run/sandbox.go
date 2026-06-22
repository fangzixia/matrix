package run

import (
	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
)

func (s *Service) useWorktreeSandbox() bool {
	return s.cfg.ActiveSandboxMode() == config.SandboxModeWorktree
}

func (s *Service) shouldUseSharedLock(m *models.Run) bool {
	if s.useWorktreeSandbox() && m.SandboxPath != "" {
		return false
	}
	if !s.useWorktreeSandbox() {
		return true
	}
	return m.SandboxPath == ""
}

func (s *Service) mergeStatusAfterRun(m *models.Run, status string) string {
	if status != "succeeded" || !s.useWorktreeSandbox() || m.SandboxPath == "" {
		return ""
	}
	if m.RunBranch != "" {
		return "pending"
	}
	return ""
}
