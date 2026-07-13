package view

import (
	agui "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// StateSnapshot 由宿主投影 RunViewState 后发出 STATE_SNAPSHOT。
func StateSnapshot(state RunViewState) agui.Event {
	return agui.NewStateSnapshotEvent(state)
}

// JobRunFinishedPayload 是 Matrix Job 终态 SSE 载荷。
type JobRunFinishedPayload struct {
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// JobRunFinished 在 Matrix Job 终态时由宿主发出（非 AG-UI RUN_FINISHED）。
func JobRunFinished(jobID string, payload JobRunFinishedPayload) agui.Event {
	return agui.NewCustomEvent("job_run_finished", agui.WithValue(map[string]any{
		"jobId":  jobID,
		"status": payload.Status,
		"output": payload.Output,
		"error":  payload.Error,
	}))
}
