package tasks

import "strings"

const implementPreset = `【编码实现】
根据需求文档在工作区内完成实现与必要自测，满足全部验收标准。`

// BuildImplementTask 组装「编码实现」任务 prompt。
func BuildImplementTask(userInput, filePath string) string {
	var b strings.Builder
	b.WriteString(implementPreset)
	b.WriteString("\n\n")
	b.WriteString(filePart("需求文件", filePath, "未指定（请使用最新的 .spec/REQ-*.md）"))
	b.WriteString("\n\n任务描述: ")
	b.WriteString(userPart(userInput, "请按需求文档完成实现。"))
	return b.String()
}
