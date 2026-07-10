package harness

import (
	"matrix/internal/ai/query"
	"matrix/internal/ai/tools"
)

// BuildMessages 按阶段 kind 组装首条 user 消息。
// planSelectedPath 为用户选中的计划逻辑路径（plan 阶段空表示新建）；planAbsPath 为解析后的绝对路径。
// sourceSandboxRunID 为 verify/implement 复用的实现 Run ID（未知时传空字符串）。
func BuildMessages(
	kind string,
	userMessage, planSelectedPath, planAbsPath, evalPath, sandboxDir, docsRoot, sourceSandboxRunID string,
) []query.Message {
	content := userMessage
	switch Kind(kind) {
	case KindPlan:
		content = BuildPlanTask(userMessage, planSelectedPath, planAbsPath)
	case KindImplement:
		content = BuildImplementTask(userMessage, planAbsPath, evalPath)
	case KindVerify:
		content = BuildVerifyTask(userMessage, planAbsPath)
	}
	if kind != "chat" && (sandboxDir != "" || docsRoot != "") {
		content = tools.FormatHarnessUserMessage(sandboxDir, docsRoot, content, sourceSandboxRunID)
	}
	return []query.Message{{Role: "user", Content: content}}
}
