package tasks

import "strings"

const buildPreset = `【完整构建】
按需求完成实现与验收闭环，评测综合分须 ≥ 8.0，未达标须说明差距与遗留项。`

// BuildBuildTask 组装「完整构建」任务 prompt。
func BuildBuildTask(userInput, filePath string) string {
	var b strings.Builder
	b.WriteString(buildPreset)
	b.WriteString("\n\n")
	b.WriteString(filePart("需求文件", filePath, "未指定（请使用最新的 .spec/REQ-*.md）"))
	b.WriteString("\n\n任务描述: ")
	b.WriteString(userPart(userInput, "请按需求完成实现并通过验收（综合分 ≥ 8.0）。"))
	return b.String()
}
