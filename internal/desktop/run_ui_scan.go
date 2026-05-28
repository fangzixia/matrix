package desktop

import "matrix/internal/desktop/tasks"

// RunUIScan 执行「页面扫描」任务。
func (b *Bridge) RunUIScan(userInput string) (*RunResult, error) {
	return b.runTaskWorkflow(tasks.NewUIScanWorkflow(userInput))
}
