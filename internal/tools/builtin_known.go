package tools

import "sync"

var (
	builtinToolNamesOnce sync.Once
	builtinToolNamesSet  map[string]struct{}
)

func loadBuiltinToolNames() {
	builtinToolNamesOnce.Do(func() {
		names := DefaultRegistry().Names()
		builtinToolNamesSet = make(map[string]struct{}, len(names))
		for _, n := range names {
			builtinToolNamesSet[n] = struct{}{}
		}
	})
}

// KnownBuiltinToolNames 返回 DefaultRegistry 中的内置工具名（只读副本）。
func KnownBuiltinToolNames() []string {
	loadBuiltinToolNames()
	out := make([]string, 0, len(builtinToolNamesSet))
	for n := range builtinToolNamesSet {
		out = append(out, n)
	}
	return out
}

// IsKnownBuiltinTool 报告 name 是否为 Matrix 内置 Worker 工具（来自 DefaultRegistry）。
func IsKnownBuiltinTool(name string) bool {
	loadBuiltinToolNames()
	_, ok := builtinToolNamesSet[name]
	return ok
}
