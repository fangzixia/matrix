// Package activity 提供 Agent 运行进度在用户界面中的活动导向文案。
package activity

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	genericTurnSummaryRE = regexp.MustCompile(`^第 \d+ 轮( · .+)?$`)
	asyncResultDescRE    = regexp.MustCompile(`Agent "([^"]+)"`)
)

const (
	// LabelThinking 是无具体工具活动时的默认状态文案。
	LabelThinking = "思考中…"
	// LabelWaitingWorkers 是协调者等待异步 Worker 时的状态文案。
	LabelWaitingWorkers = "等待 Worker 完成…"
)

// ToolSummaryInput 是 DeriveTurnSummary 所需的工具活动摘要输入。
type ToolSummaryInput struct {
	Name       string
	Preview    string
	LiveOutput string
}

// TurnSummary 返回新一轮迭代开始时的占位标题（不含轮次号）。
func TurnSummary(turn int) string {
	_ = turn
	return LabelThinking
}

// IsGenericTurnSummary 识别无意义的占位 summary（纯轮次号或旧版跃迁格式）。
func IsGenericTurnSummary(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if strings.Contains(s, "跃迁:") {
		return true
	}
	if s == LabelThinking || s == LabelWaitingWorkers {
		return true
	}
	return genericTurnSummaryRE.MatchString(s)
}

// DeriveTurnSummary 从轮次实际活动推导用户可见标题。
func DeriveTurnSummary(turn int, tools []ToolSummaryInput, message, thinking string) string {
	if turn < 1 {
		turn = 1
	}
	var parts []string
	for _, t := range tools {
		if label := extractToolIntent(t.Name, t.Preview, t.LiveOutput); label != "" {
			parts = append(parts, label)
		}
	}
	if len(parts) > 0 {
		return truncateRunes(strings.Join(parts, " · "), 120)
	}
	if line := firstLine(message); line != "" {
		return truncateRunes(line, 80)
	}
	return LabelThinking
}

// TurnThinkingLabel 新一轮 TAOR 迭代开始、尚无工具活动时的状态文案。
func TurnThinkingLabel(turn int, transition string) string {
	_ = turn
	if transition == "stop_hook_blocking" {
		return "校验未通过，正在重试…"
	}
	return LabelThinking
}

// TurnWithToolsLabel 工具调用间隙或完成后的进度文案。
func TurnWithToolsLabel(turn, toolUseCount int) string {
	_ = turn
	if toolUseCount <= 0 {
		return LabelThinking
	}
	return fmt.Sprintf("已调用 %d 个工具", toolUseCount)
}

// SummarizeTools 将工具活动合并为一行可读摘要（用于日志与进度标题）。
func SummarizeTools(tools []ToolSummaryInput) string {
	var parts []string
	for _, t := range tools {
		if label := extractToolIntent(t.Name, t.Preview, t.LiveOutput); label != "" {
			parts = append(parts, label)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return truncateRunes(strings.Join(parts, " · "), 120)
}

// AsyncResultLabel 从 Worker 异步结果消息提取简短可读标签。
func AsyncResultLabel(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "Worker 结果"
	}
	if m := asyncResultDescRE.FindStringSubmatch(content); len(m) > 1 && strings.TrimSpace(m[1]) != "" {
		return "Worker「" + truncateRunes(strings.TrimSpace(m[1]), 60) + "」"
	}
	if strings.Contains(content, "<agent_launched>") {
		if desc := xmlTagContent(content, "description"); desc != "" {
			return "Worker「" + truncateRunes(desc, 60) + "」"
		}
	}
	if line := firstLine(content); line != "" {
		return truncateRunes(line, 80)
	}
	return "Worker 结果"
}

func xmlTagContent(s, tag string) string {
	re := regexp.MustCompile(`<` + tag + `>([^<]*)</` + tag + `>`)
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
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

func extractToolIntent(name, preview, liveOutput string) string {
	if label := intentFromLiveOutput(liveOutput); label != "" {
		return label
	}
	if label := intentFromPreview(name, preview); label != "" {
		return label
	}
	if name != "" && name != "tool" {
		return name
	}
	return ""
}

func intentFromLiveOutput(liveOutput string) string {
	line := firstLine(liveOutput)
	if line == "" {
		return ""
	}
	line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "…"))
	if line == "" {
		return ""
	}
	if strings.HasPrefix(line, "读取 ") {
		return line
	}
	if strings.HasPrefix(line, "写入 ") {
		return line
	}
	if strings.HasPrefix(line, "编辑 ") {
		return line
	}
	if strings.HasPrefix(line, "列出 ") {
		return line
	}
	if strings.HasPrefix(line, "搜索:") || strings.HasPrefix(line, "搜索：") {
		return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "搜索:"), "搜索："))
	}
	if strings.HasPrefix(line, "获取 ") {
		return line
	}
	if strings.HasPrefix(line, "子 Agent:") || strings.HasPrefix(line, "子 Agent：") {
		return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "子 Agent:"), "子 Agent："))
	}
	if strings.HasPrefix(line, "Worker:") || strings.HasPrefix(line, "Worker：") {
		return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Worker:"), "Worker："))
	}
	if strings.HasPrefix(line, "$ ") {
		return line
	}
	if strings.HasPrefix(line, "PS> ") {
		return line
	}
	if strings.HasPrefix(line, "grep ") {
		return truncateRunes(line, 80)
	}
	if strings.HasPrefix(line, "glob ") {
		return truncateRunes(line, 80)
	}
	if strings.HasPrefix(line, "MCP ") {
		return truncateRunes(line, 80)
	}
	return ""
}

func intentFromPreview(name, preview string) string {
	if preview == "" {
		return ""
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(preview), &args); err != nil {
		return intentFromPartialJSON(name, preview)
	}
	if name == "agent" {
		if desc := stringArg(args, "description"); desc != "" {
			return "委派 Worker「" + truncateRunes(desc, 60) + "」"
		}
		if prompt := stringArg(args, "prompt"); prompt != "" {
			return truncateRunes(prompt, 80)
		}
		return ""
	}
	if desc := stringArg(args, "description"); desc != "" {
		return truncateRunes(desc, 80)
	}
	switch name {
	case "read_file", "write_file", "list_dir", "str_replace_editor":
		if p := firstNonEmpty(
			stringArg(args, "target_path"),
			stringArg(args, "path"),
			stringArg(args, "file_path"),
		); p != "" {
			return formatPathIntent(name, p)
		}
	case "grep", "glob":
		if pattern := stringArg(args, "pattern"); pattern != "" {
			return fmt.Sprintf("%s %s", name, truncateRunes(pattern, 60))
		}
	case "bash", "powershell":
		if cmd := stringArg(args, "command"); cmd != "" {
			return truncateRunes(cmd, 80)
		}
	case "web_search":
		if q := stringArg(args, "query"); q != "" {
			return "搜索: " + truncateRunes(q, 60)
		}
	case "web_fetch":
		if u := stringArg(args, "url"); u != "" {
			return "获取 " + truncateRunes(u, 60)
		}
	default:
		if q := stringArg(args, "query"); q != "" {
			return truncateRunes(q, 80)
		}
	}
	return ""
}

func intentFromPartialJSON(name, preview string) string {
	for _, key := range []string{"description", "target_path", "path", "file_path", "pattern", "query", "command", "url", "prompt"} {
		re := regexp.MustCompile(`"` + key + `"` + `\s*:\s*"([^"]*)"`)
		if m := re.FindStringSubmatch(preview); len(m) > 1 && m[1] != "" {
			val := m[1]
			switch key {
			case "description":
				if name == "agent" {
					return "委派 Worker「" + truncateRunes(val, 60) + "」"
				}
				return truncateRunes(val, 80)
			case "query", "prompt":
			case "target_path", "path", "file_path":
				return formatPathIntent(name, val)
			case "pattern":
				return fmt.Sprintf("%s %s", name, truncateRunes(val, 60))
			case "command":
				return truncateRunes(val, 80)
			case "url":
				return "获取 " + truncateRunes(val, 60)
			}
		}
	}
	return ""
}

func formatPathIntent(toolName, path string) string {
	path = truncateRunes(path, 60)
	switch toolName {
	case "read_file":
		return "读取 " + path
	case "write_file":
		return "写入 " + path
	case "list_dir":
		return "列出 " + path
	case "str_replace_editor":
		return "编辑 " + path
	default:
		return path
	}
}

func stringArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}
