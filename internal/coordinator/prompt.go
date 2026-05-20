package coordinator

import (
	"fmt"
	"sort"
	"strings"
)

// BaseSystemPrompt 为基础助手指令，与 CLI main.go 的 baseSystemPrompt 一致。
const BaseSystemPrompt = `你是一个有用的 AI 助手，可以使用文件系统工具完成任务。
调用工具前简要说明意图。收到工具结果后解读并决定下一步。
掌握足够信息时，给出清晰简洁的最终答案。`

// BuildParentSystemPrompt 组装 Coordinator 父会话系统提示（基础指令 + 协调者说明 + Worker 工具上下文）。
func BuildParentSystemPrompt(workerToolNames, mcpServerNames []string) string {
	return strings.Join([]string{
		BaseSystemPrompt,
		CoordinatorSystemPrompt,
		workerUserContext(workerToolNames, mcpServerNames),
	}, "\n\n")
}

// workerUserContext 对应 coordinatorMode.ts 的 getCoordinatorUserContext。
func workerUserContext(toolNames, mcpServerNames []string) string {
	sorted := append([]string(nil), toolNames...)
	sort.Strings(sorted)
	s := fmt.Sprintf(
		"Workers spawned via the agent tool have access to these tools: %s",
		strings.Join(sorted, ", "),
	)
	if len(mcpServerNames) > 0 {
		mcp := append([]string(nil), mcpServerNames...)
		sort.Strings(mcp)
		s += fmt.Sprintf(
			"\n\nWorkers also have access to MCP tools from connected MCP servers: %s",
			strings.Join(mcp, ", "),
		)
	}
	return s
}
