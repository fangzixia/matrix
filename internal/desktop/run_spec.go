package desktop

import "matrix/internal/desktop/tasks"

// RunSpec 执行「创建需求」任务。
func (b *Bridge) RunSpec(userInput, filePath string) (*RunResult, error) {
	return b.RunAgentSession(tasks.BuildSpecTask(userInput, filePath))
}
