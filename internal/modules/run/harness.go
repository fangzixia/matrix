package run

import (
	"matrix/internal/ai/harness"
	"matrix/internal/ai/query"
	"matrix/internal/ai/tools"
)

// BuildHarnessMessages 按 harness kind 组装首条 user 消息。
// planSelectedPath 为用户选中的计划逻辑路径（plan 阶段空表示新建）；planAbsPath 为解析后的绝对路径。
// sourceSandboxRunID 为 verify/implement 复用的实现 Run ID（未知时传空字符串）。
func BuildHarnessMessages(kind *Kind, userMessage, planSelectedPath, planAbsPath, evalPath, sandboxDir, docsRoot, sourceSandboxRunID string) []query.Message {
	content := userMessage
	switch *kind {
	case KindPlan:
		content = harness.BuildPlanTask(userMessage, planSelectedPath, planAbsPath)
	case KindImplement:
		content = harness.BuildImplementTask(userMessage, planAbsPath, evalPath)
	case KindVerify:
		content = harness.BuildVerifyTask(userMessage, planAbsPath)
	case KindChat:
	default:
	}
	if *kind != KindChat && (sandboxDir != "" || docsRoot != "") {
		content = tools.FormatHarnessUserMessage(sandboxDir, docsRoot, content, sourceSandboxRunID)
	}
	return []query.Message{{Role: "user", Content: content}}
}
