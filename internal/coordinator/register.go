package coordinator

import "matrix/internal/tools"

// RegisterTools 将 Coordinator 编排工具注册到 parent 工具表。
func RegisterTools(reg *tools.Registry, cfg Config) {
	reg.Register(NewAgentTool(cfg))
	reg.Register(NewSendMessageTool(cfg))
	reg.Register(NewTaskStopTool(cfg))
}

// CloneWorkerRegistry 复制 base 中除 Coordinator 专用工具外的所有工具，供 Worker 使用。
func CloneWorkerRegistry(base *tools.Registry) *tools.Registry {
	out := tools.NewRegistry()
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
