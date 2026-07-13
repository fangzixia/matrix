package harness

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FormatWorkspaceUserMessage 为 user 消息附加工作区与文档目录前缀（宿主编排，非 SDK tools）。
// sourceSandboxRunID 非空时表示 verify/implement 复用的实现 Run ID。
func FormatWorkspaceUserMessage(codeSandbox, docsRoot, msg, sourceSandboxRunID string) string {
	msg = strings.TrimSpace(msg)
	var lines []string
	if root := normalizeRoot(codeSandbox); root != "" {
		lines = append(lines, fmt.Sprintf("沙箱目录（源代码）: %s", root))
	}
	if id := strings.TrimSpace(sourceSandboxRunID); id != "" && id != "00000000-0000-0000-0000-000000000000" {
		lines = append(lines, fmt.Sprintf("实现 Run（代码复制来源）: %s", id))
	}
	if root := normalizeRoot(docsRoot); root != "" {
		lines = append(lines, fmt.Sprintf("文档目录（计划/评测，非源码）: %s", root))
	}
	if len(lines) == 0 {
		return msg
	}
	prefix := strings.Join(lines, "\n")
	if msg == "" {
		return prefix
	}
	return prefix + "\n\n" + msg
}

func normalizeRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" || root == "." {
		return ""
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}
	return filepath.Clean(abs)
}
