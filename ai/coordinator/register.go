package coordinator

import (
	"matrix/ai/util"
)

// RegisterTools 将 Coordinator 编排工具注册到 parent 工具表。
func RegisterTools(reg *util.Registry, cfg Config) {
	reg.Register(NewAgentTool(cfg))
	reg.Register(NewSendMessageTool(cfg))
	reg.Register(NewTaskStopTool(cfg))
}

// CloneWorkerRegistry 复制 base 中除 Coordinator 专用工具外的所有工具，供 Worker 使用。
func CloneWorkerRegistry(base *util.Registry) *util.Registry {
	out := util.NewRegistry()
	if base == nil {
		return out
	}
	for _, name := range base.Names() {
		if _, skip := ParentToolNames[name]; skip {
			continue
		}
		if t := base.Get(name); t != nil {
			out.Register(t)
		}
	}
	return out
}

// BuildWorkerRegistry 构造 Worker 工具表（不含 Coordinator 编排工具）。
func BuildWorkerRegistry(base *util.Registry) *util.Registry {
	return CloneWorkerRegistry(base)
}
