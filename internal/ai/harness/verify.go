package harness

import "matrix/internal/modules/workspace"

const verifyPreset = `【验收评测】
对照计划与代码产出评测报告（docs/evaluations/EVAL-PLAN-*-*.md），每条 AC 给出结论与依据，含综合分（10 分制，≥8.0 为通过）。`

// NewVerifyWorkflow 创建「验收评测」流水线工作流。
func NewVerifyWorkflow(userInput, filePath string) Workflow {
	return Workflow{
		Kind:         KindVerify,
		State:        StatePrepared,
		Preset:       verifyPreset,
		UserInput:    userInput,
		FilePath:     filePath,
		FileLabel:    "计划文档",
		FileFallback: "未指定（请使用最新的 docs/plans/PLAN-*.md）",
		DefaultTask:  "请对照计划验收当前实现，并生成评测报告。",
		ExpectedArtifacts: []string{
			workspace.DocsEvaluationsRel + "/EVAL-PLAN-*-*.md",
		},
		Acceptance: []string{
			"每条 AC 有通过/失败结论和证据",
			"综合分为 10 分制，8.0 及以上为通过",
		},
		Recovery: []string{
			"验收不通过时列出差距和可复测步骤",
		},
	}
}

// BuildVerifyTask 组装「验收评测」任务 prompt。
func BuildVerifyTask(userInput, filePath string) string {
	return NewVerifyWorkflow(userInput, filePath).Prompt()
}
