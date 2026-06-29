package run

import (
	"matrix/internal/ai/harness"
	"matrix/internal/ai/query"
	"matrix/internal/ai/tools"
)

// BuildHarnessMessages 按 harness kind 组装首条 user 消息。
// planSelectedPath 为用户选中的计划逻辑路径（plan 阶段空表示新建）；planAbsPath 为解析后的绝对路径。
func BuildHarnessMessages(kind, userMessage, planSelectedPath, planAbsPath, evalPath, sandboxDir, docsRoot string) []query.Message {
	content := userMessage
	switch kind {
	case string(harness.KindPlan):
		content = harness.BuildPlanTask(userMessage, planSelectedPath, planAbsPath)
	case string(harness.KindImplement):
		content = harness.BuildImplementTask(userMessage, planAbsPath)
	case string(harness.KindVerify):
		content = harness.BuildVerifyTask(userMessage, planAbsPath)
	case string(harness.KindBuild):
		content = harness.BuildBuildTask(userMessage, planAbsPath, evalPath)
	case "chat", "task", "":
	default:
	}
	if kind != "chat" && kind != "task" && kind != "" && (sandboxDir != "" || docsRoot != "") {
		content = tools.FormatHarnessUserMessage(sandboxDir, docsRoot, content)
	}
	return []query.Message{{Role: "user", Content: content}}
}
