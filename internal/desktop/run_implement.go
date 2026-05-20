package desktop

import "matrix/internal/desktop/tasks"

// RunImplement 执行「编码实现」任务。
func (b *Bridge) RunImplement(userInput, filePath string) (*RunResult, error) {
	return b.RunAgentSession(tasks.BuildImplementTask(userInput, filePath))
}
