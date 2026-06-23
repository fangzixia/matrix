package run

import (
	"matrix/internal/ai/harness"
	"matrix/internal/ai/query"
	"matrix/internal/ai/tools"
)

// BuildHarnessMessages 按 harness kind 组装首条 user 消息。
func BuildHarnessMessages(kind, userMessage, planPath, evalPath, sandboxDir, docsRoot string) []query.Message {
	content := userMessage
	switch kind {
	case string(harness.KindPlan):
		content = harness.BuildPlanTask(userMessage, planPath)
	case string(harness.KindImplement):
		content = harness.BuildImplementTask(userMessage, planPath)
	case string(harness.KindVerify):
		content = harness.BuildVerifyTask(userMessage, planPath)
	case string(harness.KindBuild):
		content = harness.BuildBuildTask(userMessage, planPath, evalPath)
	case "chat", "task", "":
	default:
	}
	if kind != "chat" && kind != "task" && kind != "" && (sandboxDir != "" || docsRoot != "") {
		content = tools.FormatHarnessUserMessage(sandboxDir, docsRoot, content)
	}
	return []query.Message{{Role: "user", Content: content}}
}
