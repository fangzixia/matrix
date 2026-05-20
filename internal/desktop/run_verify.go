package desktop

import "matrix/internal/desktop/tasks"

// RunVerify 执行「验收评测」任务。
func (b *Bridge) RunVerify(userInput, filePath string) (*RunResult, error) {
	return b.RunAgentSession(tasks.BuildVerifyTask(userInput, filePath))
}
