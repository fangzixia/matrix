package run

import (
	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
)

// useWorktreeSandbox 判断当前配置是否使用 worktree 沙箱。
func (s *Service) useWorktreeSandbox() bool {
	return s.runtimeCfg.ActiveSandboxMode() == config.SandboxModeWorktree
}

// shouldUseSharedLock 判断是否应对沙箱加共享锁。
func (s *Service) shouldUseSharedLock(m *models.Run) bool {
	if s.useWorktreeSandbox() && m.SandboxPath != "" {
		return false
	}
	if !s.useWorktreeSandbox() {
		return true
	}
	return m.SandboxPath == ""
}

// mergeStatusAfterRun 根据合并结果更新 Run 状态。
func (s *Service) mergeStatusAfterRun(m *models.Run, status string) string {
	if status != "succeeded" || !s.useWorktreeSandbox() || m.SandboxPath == "" {
		return ""
	}
	if !UsesWorktreeKind(m.Kind) && m.Kind != "pipeline" {
		return ""
	}
	if m.RunBranch != "" {
		return "pending"
	}
	return ""
}
