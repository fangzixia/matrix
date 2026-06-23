package harness

import "matrix/internal/modules/workspace"

const buildPreset = `【完整构建】
按计划完成实现与验收闭环，评测综合分 ≥ 8.0，未达标须说明差距与遗留项。`

// NewBuildWorkflow 创建「完整构建」流水线工作流。
func NewBuildWorkflow(userInput, planPath, evalPath string) Workflow {
	w := Workflow{
		Kind:         KindBuild,
		State:        StatePrepared,
		Preset:       buildPreset,
		UserInput:    userInput,
		FilePath:     planPath,
		FileLabel:    "计划文档",
		FileFallback: "未指定（请使用最新的 docs/plans/PLAN-*.md）",
		DefaultTask:  "请按计划完成实现并通过验收（综合分 ≥ 8.0）。",
		ExpectedArtifacts: []string{
			"代码实现",
			workspace.DocsEvaluationsRel + "/EVAL-PLAN-*-*.md",
		},
		Acceptance: []string{
			"实现满足验收标准",
			"评测综合分 >= 8.0",
		},
		Recovery: []string{
			"未达标时说明差距、遗留项和下一轮修复建议",
		},
	}
	if evalPath != "" {
		w.SecondaryFilePath = evalPath
		w.SecondaryFileLabel = "基准评测报告"
		w.SecondaryFileFallback = "未指定基准评测"
	}
	return w
}

// BuildBuildTask 组装「完整构建」任务 prompt。
func BuildBuildTask(userInput, planPath, evalPath string) string {
	return NewBuildWorkflow(userInput, planPath, evalPath).Prompt()
}
