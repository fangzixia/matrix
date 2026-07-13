package harness

const implementPreset = `【编码实现】
根据计划文档在工作区内完成实现与必要自测。以计划文档「附录：技术验收标准」为实现与自测依据（若计划无附录章节，则使用「验收标准」章节）；正文为用户向说明，供理解背景与范围。若存在与计划对应的评测报告，请参考其中未通过项与修复建议。

implement 面向附录 AC 完成开发与开发者自测（单测/构建）；verify 将以系统用户身份独立做黑盒验收，impl 自测不能替代 verify。`

// NewImplementWorkflow 创建「编码实现」流水线工作流。
func NewImplementWorkflow(userInput, planPath, evalPath string) Workflow {
	w := Workflow{
		Kind:         KindImplement,
		State:        StatePrepared,
		Preset:       implementPreset,
		UserInput:    userInput,
		FilePath:     planPath,
		FileLabel:    "计划文档",
		FileFallback: "未指定（请使用最新的 docs/plans/PLAN-*.md）",
		DefaultTask:  "请按计划文档完成实现。",
		ExpectedArtifacts: []string{
			"满足计划的代码变更",
			"必要的测试或验证输出",
		},
		Acceptance: []string{
			"逐条满足计划附录（或验收标准）中的 AC",
			"相关测试或构建命令已执行并报告结果",
		},
		Recovery: []string{
			"实现失败时说明阻塞点、已改文件和下一步",
		},
	}
	if evalPath != "" {
		w.SecondaryFilePath = evalPath
		w.SecondaryFileLabel = "评测报告"
	}
	return w
}

// BuildImplementTask 组装「编码实现」任务 prompt。
func BuildImplementTask(userInput, planPath, evalPath string) string {
	return NewImplementWorkflow(userInput, planPath, evalPath).Prompt()
}
