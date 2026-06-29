// Package activity 提供 Agent 运行进度在用户界面中的活动导向文案。
package activity

import "fmt"

// TurnSummary 返回轮次在用户界面中的简短标题（不含内部跃迁信息）。
func TurnSummary(turn int) string {
	if turn < 1 {
		turn = 1
	}
	return fmt.Sprintf("第 %d 轮", turn)
}

// TurnThinkingLabel 新一轮 TAOR 迭代开始、尚无工具活动时的状态文案。
func TurnThinkingLabel(turn int, transition string) string {
	if transition == "stop_hook_blocking" {
		return fmt.Sprintf("第 %d 轮 · 校验未通过，正在重试…", turn)
	}
	if turn <= 1 {
		return "思考中…"
	}
	return fmt.Sprintf("第 %d 轮 · 思考中…", turn)
}

// TurnWithToolsLabel 工具调用间隙或完成后的轮次进度文案。
func TurnWithToolsLabel(turn, toolUseCount int) string {
	if toolUseCount <= 0 {
		return TurnThinkingLabel(turn, "")
	}
	if turn <= 1 {
		return fmt.Sprintf("已调用 %d 个工具", toolUseCount)
	}
	return fmt.Sprintf("第 %d 轮 · 已调用 %d 个工具", turn, toolUseCount)
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
