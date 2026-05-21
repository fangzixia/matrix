package tasks

import (
	"path/filepath"
	"strings"
)

const uiScanPreset = ``

// BuildUIScanTask 组装「页面扫描」任务 prompt。
func BuildUIScanTask(userInput string) string {
	outDir := filepath.Join(".spec", "pagescan")
	var b strings.Builder
	b.WriteString(uiScanPreset)
	b.WriteString("\n\n输出目录: ")
	b.WriteString(outDir)
	b.WriteString("\n\n扫描说明:\n")
	b.WriteString(userPart(userInput, "请根据我提供的访问地址与登录信息，生成完整的 Web 应用页面结构扫描报告。"))
	return b.String()
}
