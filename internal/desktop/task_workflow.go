package desktop

import "matrix/internal/desktop/tasks"

func (b *Bridge) runTaskWorkflow(workflow tasks.Workflow) (*RunResult, error) {
	workflow = workflow.WithState(tasks.StateRunning)
	result, err := b.RunAgentSession(workflow.Prompt())
	if result != nil {
		result.TaskKind = string(workflow.Kind)
		if result.HasError {
			result.TaskState = string(tasks.StateFailed)
		} else {
			result.TaskState = string(tasks.StateSucceeded)
		}
	}
	return result, err
}
