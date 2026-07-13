package harness

import (
	"fmt"
	"strings"
)

const docsPlansRel = "docs/plans"
const docsEvaluationsRel = "docs/evaluations"

// trim 去除字符串首尾空白。
func trim(s string) string {
	return strings.TrimSpace(s)
}

// userPart 构造 Harness 用户消息片段。
func userPart(userInput, defaultTask string) string {
	if t := trim(userInput); t != "" {
		return t
	}
	return defaultTask
}

// filePart 构造 Harness 文件引用消息片段。
func filePart(label, filePath, ifMissing string) string {
	if filePath != "" {
		return fmt.Sprintf("%s: %s", label, filePath)
	}
	return fmt.Sprintf("%s: %s", label, ifMissing)
}
