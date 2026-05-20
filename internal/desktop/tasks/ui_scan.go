package tasks

import (
	"path/filepath"
	"strings"
)

const uiScanPreset = `【页面扫描】
使用 browser_* MCP 工具访问用户提供的应用，采集真实页面结构并写入 .spec/pagescan/ 报告（PAGESCAN-时间戳.md）；勿编造未访问页面。`

// BuildUIScanTask 组装「页面扫描」任务 prompt。
func BuildUIScanTask(userInput, filePath string) string {
	outDir := filepath.Join(".spec", "pagescan")
	var b strings.Builder
	b.WriteString(uiScanPreset)
	b.WriteString("\n\n输出目录: ")
	b.WriteString(outDir)
	if filePath != "" {
		b.WriteString("\n\n附加参考: ")
		b.WriteString(filePath)
	}
	b.WriteString("\n\n扫描说明:\n")
	b.WriteString(userPart(userInput, "请根据我提供的访问地址与登录信息，生成完整的 Web 应用页面结构扫描报告。"))
	return b.String()
}
