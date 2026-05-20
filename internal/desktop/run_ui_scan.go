package desktop

import "matrix/internal/desktop/tasks"

// RunUIScan 执行「页面扫描」任务。
func (b *Bridge) RunUIScan(userInput, filePath string) (*RunResult, error) {
	return b.RunAgentSession(tasks.BuildUIScanTask(userInput, filePath))
}
