package view

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	reLoopModel = regexp.MustCompile(`(?i)^loop:\s*模型错误:\s*`)
	reLLM       = regexp.MustCompile(`(?i)^llm:\s*`)
	reServerErr = regexp.MustCompile(`服务端返回 \d+:\s*(\{[\s\S]+\})`)
)

// FormatUserRunError 将后端/模型原始错误转为用户可读文案。
func FormatUserRunError(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "任务执行失败，请稍后重试"
	}
	msg := reLoopModel.ReplaceAllString(trimmed, "")
	msg = reLLM.ReplaceAllString(msg, "")

	if m := reServerErr.FindStringSubmatch(msg); len(m) > 1 {
		var parsed struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(m[1]), &parsed) == nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
	}

	lower := strings.ToLower(msg)
	if strings.Contains(lower, "authentication") || strings.Contains(lower, "api key") ||
		strings.Contains(lower, "invalid") && strings.Contains(lower, "key") || strings.Contains(msg, "401") {
		return "模型 API Key 无效或已过期，请在「管理区域 → 系统配置 → AI 模型」中检查配置。"
	}
	if strings.Contains(msg, "未配置模型") || strings.Contains(msg, "未配置 API Key") {
		return msg
	}
	if strings.Contains(lower, "429") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") {
		return "模型服务请求过于频繁，请稍后再试。"
	}
	if matched, _ := regexp.MatchString(`服务端返回 5\d\d|internal server error`, msg); matched {
		return "模型服务暂时不可用，请稍后再试。"
	}
	if len(msg) > 200 {
		return "模型调用失败，请检查系统配置中的 AI 模型设置。"
	}
	return msg
}
