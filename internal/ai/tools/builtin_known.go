package tools

import "sync"

var (
	builtinToolNamesOnce sync.Once
	builtinToolNamesSet  map[string]struct{}
)

// loadBuiltinToolNames 加载内置工具名称列表。
func loadBuiltinToolNames() {
	builtinToolNamesOnce.Do(func() {
		names := DefaultRegistry().Names()
		builtinToolNamesSet = make(map[string]struct{}, len(names))
		for _, n := range names {
			builtinToolNamesSet[n] = struct{}{}
		}
	})
}

// IsKnownBuiltinTool 报告 name 是否为 Matrix 内置 Worker 工具（来自 DefaultRegistry）。
func IsKnownBuiltinTool(name string) bool {
	loadBuiltinToolNames()
	_, ok := builtinToolNamesSet[name]
	return ok
}
