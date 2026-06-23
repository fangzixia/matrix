package harness

const implementPreset = `【编码实现】
根据计划文档在工作区内完成实现与必要自测，满足全部验收标准。`

// NewImplementWorkflow 创建「编码实现」流水线工作流。
func NewImplementWorkflow(userInput, filePath string) Workflow {
	return Workflow{
		Kind:         KindImplement,
		State:        StatePrepared,
		Preset:       implementPreset,
		UserInput:    userInput,
		FilePath:     filePath,
		FileLabel:    "计划文档",
		FileFallback: "未指定（请使用最新的 docs/plans/PLAN-*.md）",
		DefaultTask:  "请按计划文档完成实现。",
		ExpectedArtifacts: []string{
			"满足计划的代码变更",
			"必要的测试或验证输出",
		},
		Acceptance: []string{
			"逐条满足计划验收标准",
			"相关测试或构建命令已执行并报告结果",
		},
		Recovery: []string{
			"实现失败时说明阻塞点、已改文件和下一步",
		},
	}
}

// BuildImplementTask 组装「编码实现」任务 prompt。
func BuildImplementTask(userInput, filePath string) string {
	return NewImplementWorkflow(userInput, filePath).Prompt()
}
