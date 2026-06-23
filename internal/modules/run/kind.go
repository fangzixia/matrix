package run

// UsesWorktreeKind 报告 Harness 阶段是否在独立 worktree 中修改代码。
func UsesWorktreeKind(kind string) bool {
	switch kind {
	case "implement", "build":
		return true
	default:
		return false
	}
}

// RequiresApprovedPlan 报告启动该阶段是否需要已批准的计划文件。
func RequiresApprovedPlan(kind string) bool {
	switch kind {
	case "implement", "verify", "build", "pipeline":
		return true
	default:
		return false
	}
}

// RequiresPlanFile 报告启动该阶段时是否必须设置 file_path。
func RequiresPlanFile(kind string) bool {
	switch kind {
	case "implement", "verify", "build", "pipeline":
		return true
	default:
		return false
	}
}
