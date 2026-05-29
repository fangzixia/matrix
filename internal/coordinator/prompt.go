package coordinator

import (
	"fmt"
	"sort"
	"strings"
)

// BaseSystemPrompt 为基础助手指令，供非 Coordinator 场景（如将来 CLI 直连 Worker）复用。
// Coordinator 父会话请使用 [ParentBaseSystemPrompt]。
const BaseSystemPrompt = `你是一个有用的 AI 助手，可以使用文件系统工具完成任务。
调用工具前简要说明意图。收到工具结果后解读并决定下一步。
掌握足够信息时，给出清晰简洁的最终答案。`

// ParentBaseSystemPrompt 是 Coordinator 父会话的简短角色说明（不含文件系统工具能力）。
const ParentBaseSystemPrompt = `你是一个 AI 任务协调者，帮助用户达成目标。
你不直接读写文件、搜索路径或执行 shell；此类操作必须通过 agent 工具委派 Worker。
调用工具前简要说明意图；掌握足够信息时，向用户给出清晰简洁的最终答案。`

// BuildParentSystemPrompt 组装 Coordinator 父会话系统提示（协调者说明 + Worker 工具上下文）。
func BuildParentSystemPrompt(workerToolNames, mcpServerNames []string) string {
	return strings.Join([]string{
		ParentBaseSystemPrompt,
		CoordinatorSystemPrompt,
		workerUserContext(workerToolNames, mcpServerNames),
	}, "\n\n")
}

// workerUserContext 对应 coordinatorMode.ts 的 getCoordinatorUserContext。
func workerUserContext(toolNames, mcpServerNames []string) string {
	sorted := append([]string(nil), toolNames...)
	sort.Strings(sorted)
	s := fmt.Sprintf(
		"下列工具仅 Worker 可用，你不可直接调用：%s。若需读文件、搜路径或跑命令，请通过 agent 工具委派，并在 prompt 中写清路径、模式与验收标准。",
		strings.Join(sorted, ", "),
	)
	if len(mcpServerNames) > 0 {
		mcp := append([]string(nil), mcpServerNames...)
		sort.Strings(mcp)
		s += fmt.Sprintf(
			"\n\n下列 MCP 服务器仅 Worker 经 MCP 工具调用，你不可直接调用：%s。",
			strings.Join(mcp, ", "),
		)
	}
	return s
}
