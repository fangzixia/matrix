package tasks

import "strings"

const specPreset = `【创建需求】
根据用户描述与所选文件（如有），在工作区 .spec/ 下编写或更新需求文档（REQ-五位序号.md）。
须包含：需求目标（业务语言）、可验证的验收标准（AC-*），勿编造未给出的指标。`

// BuildSpecTask 组装「创建需求」任务的 prompt 正文（不含工作区前缀，由 Bridge.formatUserMessage 追加）。
func BuildSpecTask(userInput, filePath string) string {
	var b strings.Builder
	b.WriteString(specPreset)
	b.WriteString("\n\n")
	b.WriteString(filePart("目标需求文件", filePath, "未指定（请新建 REQ-xxxxx.md）"))
	b.WriteString("\n\n任务描述: ")
	b.WriteString(userPart(userInput, "请根据我的描述编写需求文档。"))
	return b.String()
}
