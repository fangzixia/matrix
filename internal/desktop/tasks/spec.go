package tasks

const specPreset = `【创建需求】
根据用户描述与所选文件（如有），在工作区 .matrix/ 下编写或更新需求文档（REQ-五位序号.md）。
须包含：需求目标（业务语言）、可验证的验收标准（AC-*），勿编造未给出的指标。`

func NewSpecWorkflow(userInput, filePath string) Workflow {
	return Workflow{
		Kind:         KindSpec,
		State:        StatePrepared,
		Preset:       specPreset,
		UserInput:    userInput,
		FilePath:     filePath,
		FileLabel:    "目标需求文件",
		FileFallback: "未指定（请新建 REQ-xxxxx.md）",
		DefaultTask:  "请根据我的描述编写需求文档。",
		ExpectedArtifacts: []string{
			".matrix/REQ-xxxxx.md",
		},
		Acceptance: []string{
			"需求目标使用业务语言描述",
			"每条验收标准以 AC-* 标识且可验证",
		},
		Recovery: []string{
			"信息不足时列出缺口，不要编造指标",
		},
	}
}

// BuildSpecTask 组装「创建需求」任务的 prompt 正文（不含工作区前缀，由 Bridge.formatUserMessage 追加）。
func BuildSpecTask(userInput, filePath string) string {
	return NewSpecWorkflow(userInput, filePath).Prompt()
}
