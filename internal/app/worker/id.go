// Package worker 提供嵌入式任务 Worker 的进程标识。
package worker

import "os"

// ID 返回当前 Worker 实例标识（优先主机名，否则为 embedded）。
func ID() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "embedded"
}
