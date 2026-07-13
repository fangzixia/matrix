package query

import (
	"fmt"
	"strings"

	"matrix/ai/llm"
)

const labelThinking = "思考中…"

// LabelWaitingWorkers 是协调者等待异步 Worker 时的固定状态文案。
const LabelWaitingWorkers = "等待 Worker 完成…"

// TurnThinkingLabel 新一轮 TAOR 迭代开始时的状态文案。
func TurnThinkingLabel(turn int, transition string) string {
	_ = turn
	if transition == "stop_hook_blocking" {
		return "校验未通过，正在重试…"
	}
	return labelThinking
}

// SummarizeToolCalls 将工具调用合并为一行可读摘要（用于日志）。
func SummarizeToolCalls(calls []llm.ToolCall) string {
	if len(calls) == 0 {
		return ""
	}
	names := make([]string, 0, len(calls))
	for _, tc := range calls {
		if tc.Function.Name != "" {
			names = append(names, tc.Function.Name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return strings.Join(names, ", ")
}

// SummarizeSingleTool 返回单工具活动的简短标签。
func SummarizeSingleTool(name, preview string) string {
	if name != "" {
		return name
	}
	return PreviewText(preview, 40)
}

// AsyncResultLabel 从 Worker 异步结果消息提取简短可读标签。
func AsyncResultLabel(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "Worker 结果"
	}
	if i := strings.IndexAny(content, "\r\n"); i >= 0 {
		content = strings.TrimSpace(content[:i])
	}
	if len(content) > 80 {
		content = PreviewText(content, 80)
	}
	return content
}

// ToolActivityLabel 根据工具名与执行状态生成活动导向文案。
func ToolActivityLabel(toolName, status string) string {
	if toolName == "" {
		return ""
	}
	switch status {
	case "started", "input_streaming":
		return fmt.Sprintf("正在调用 %s", toolName)
	case "streaming":
		return fmt.Sprintf("%s · 输出中…", toolName)
	case "completed", "success":
		return fmt.Sprintf("%s · 已完成", toolName)
	case "failed":
		return fmt.Sprintf("%s · 执行失败", toolName)
	default:
		return fmt.Sprintf("正在调用 %s", toolName)
	}
}

// TurnWithToolsLabel 工具调用完成后的进度文案。
func TurnWithToolsLabel(turn, toolUseCount int) string {
	_ = turn
	if toolUseCount <= 0 {
		return labelThinking
	}
	return fmt.Sprintf("已调用 %d 个工具", toolUseCount)
}
