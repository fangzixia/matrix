package run

import (
	"matrix/internal/ai/harness"
	"matrix/internal/ai/query"
	"matrix/internal/ai/tools"
)

// BuildHarnessMessages 按 harness kind 组装首条 user 消息；sandboxDir 非空时附加沙箱路径前缀。
func BuildHarnessMessages(kind, userMessage, filePath, sandboxDir string) []query.Message {
	content := userMessage
	switch kind {
	case string(harness.KindPlan):
		content = harness.BuildPlanTask(userMessage, filePath)
	case string(harness.KindImplement):
		content = harness.BuildImplementTask(userMessage, filePath)
	case string(harness.KindVerify):
		content = harness.BuildVerifyTask(userMessage, filePath)
	case string(harness.KindBuild):
		content = harness.BuildBuildTask(userMessage, filePath)
	case string(harness.KindUIScan):
		content = harness.BuildUIScanTask(userMessage)
	case "chat", "task", "":
	default:
	}
	if kind != "chat" && kind != "task" && kind != "" && sandboxDir != "" {
		content = tools.FormatWorkerUserMessage(sandboxDir, content)
	}
	return []query.Message{{Role: "user", Content: content}}
}
