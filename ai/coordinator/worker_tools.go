package coordinator

import (
	"fmt"
	"matrix/ai/tools"
	"sync"
)

var (
	workerBuiltinNamesOnce sync.Once
	workerBuiltinNames     map[string]struct{}
)

func loadWorkerBuiltinNames() {
	workerBuiltinNamesOnce.Do(func() {
		names := tools.DefaultRegistry().Names()
		workerBuiltinNames = make(map[string]struct{}, len(names))
		for _, n := range names {
			workerBuiltinNames[n] = struct{}{}
		}
	})
}

// IsWorkerBuiltinTool 报告 name 是否为 Worker 内置文件/命令类工具。
func IsWorkerBuiltinTool(name string) bool {
	loadWorkerBuiltinNames()
	_, ok := workerBuiltinNames[name]
	return ok
}

// WorkerOnlyToolMessage 当父会话误调 Worker 工具时的提示文案。
func WorkerOnlyToolMessage(name string) string {
	return fmt.Sprintf(
		"工具 %q 仅 Worker 可用。请使用 agent 工具委派，在 prompt 中写明路径、模式和验收标准。",
		name,
	)
}
