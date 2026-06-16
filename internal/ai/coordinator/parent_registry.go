package coordinator

import (
	"strings"

	"matrix/internal/ai/tools"
)

// ParentToolNames 为 Coordinator 父会话可见的内置工具名。
// 协调者模式允许的工具：agent / send_message / task_stop。
var ParentToolNames = map[string]struct{}{
	"agent":        {},
	"send_message": {},
	"task_stop":    {},
}

// IsParentAllowedTool 判断工具名是否允许出现在 Coordinator 父会话。
// MCP 工具中，PR 订阅类由父级直接调用（对齐 toolPool.isPrActivitySubscriptionTool）。
func IsParentAllowedTool(name string) bool {
	if _, ok := ParentToolNames[name]; ok {
		return true
	}
	return strings.HasSuffix(name, "subscribe_pr_activity") ||
		strings.HasSuffix(name, "unsubscribe_pr_activity")
}

// NewParentRegistry 创建仅含编排工具的父会话注册表，不包含 Worker 的文件/命令类工具。
func NewParentRegistry(cfg Config) *tools.Registry {
	reg := tools.NewRegistry()
	RegisterTools(reg, cfg)
	return reg
}
