package harness

const buildPreset = `【完整构建】
按需求完成实现与验收闭环，评测综合分 ≥ 8.0，未达标须说明差距与遗留项。`

// NewBuildWorkflow 创建「完整构建」流水线工作流。
func NewBuildWorkflow(userInput, filePath string) Workflow {
	return Workflow{
		Kind:         KindBuild,
		State:        StatePrepared,
		Preset:       buildPreset,
		UserInput:    userInput,
		FilePath:     filePath,
		FileLabel:    "需求文档",
		FileFallback: "未指定（请使用最新的 .matrix/SPEC-*.md）",
		DefaultTask:  "请按需求完成实现并通过验收（综合分 ≥ 8.0）。",
		ExpectedArtifacts: []string{
			"代码实现",
			".matrix/EVAL-SPEC-*-*.md",
		},
		Acceptance: []string{
			"实现满足验收标准",
			"评测综合分 >= 8.0",
		},
		Recovery: []string{
			"未达标时说明差距、遗留项和下一轮修复建议",
		},
	}
}

// BuildBuildTask 组装「完整构建」任务 prompt。
func BuildBuildTask(userInput, filePath string) string {
	return NewBuildWorkflow(userInput, filePath).Prompt()
}
