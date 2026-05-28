package desktop

import "matrix/internal/desktop/tasks"

// RunBuild 执行「完整构建」任务。
func (b *Bridge) RunBuild(userInput, filePath string) (*RunResult, error) {
	return b.runTaskWorkflow(tasks.NewBuildWorkflow(userInput, filePath))
}
