package harness

import (
	"path/filepath"
)

const uiScanPreset = ``

func NewUIScanWorkflow(userInput string) Workflow {
	outDir := filepath.Join(".matrix", "pagescan")
	return Workflow{
		Kind:         KindUIScan,
		State:        StatePrepared,
		Preset:       uiScanPreset,
		UserInput:    userInput,
		FileLabel:    "输出目录",
		FilePath:     outDir,
		FileFallback: outDir,
		DefaultTask:  "请根据我提供的访问地址与登录信息，生成完整的 Web 应用页面结构扫描报告。",
		ExpectedArtifacts: []string{
			filepath.Join(outDir, "页面结构扫描报告"),
		},
		Acceptance: []string{
			"覆盖用户提供的关键页面和登录路径",
			"报告页面结构、交互入口和异常状态",
		},
		Recovery: []string{
			"访问失败时记录 URL、错误和可复测步骤",
		},
	}
}

// BuildUIScanTask 组装「页面扫描」任务 prompt。
func BuildUIScanTask(userInput string) string {
	return NewUIScanWorkflow(userInput).Prompt()
}
