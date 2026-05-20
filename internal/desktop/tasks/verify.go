package tasks

import "strings"

const verifyPreset = `【验收评测】
对照需求与代码产出评测报告（.spec/EVAL-REQ-*-*.md），每条 AC 给出结论与依据，含综合分（10 分制，≥8.0 为通过）。`

// BuildVerifyTask 组装「验收评测」任务 prompt。
func BuildVerifyTask(userInput, filePath string) string {
	var b strings.Builder
	b.WriteString(verifyPreset)
	b.WriteString("\n\n")
	b.WriteString(filePart("需求文件", filePath, "未指定（请使用最新的 .spec/REQ-*.md）"))
	b.WriteString("\n\n任务描述: ")
	b.WriteString(userPart(userInput, "请对照需求验收当前实现，并生成评测报告。"))
	return b.String()
}
