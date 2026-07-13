package view

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CatchUpAfterSeq 从事件日志生成 seq > afterSeq 的 SSE 事件；Job 终态时返回 done=true。
func (s *Store) CatchUpAfterSeq(
	ctx context.Context,
	jobID string,
	mode Mode,
	afterSeq int64,
	status, output, errMsg string,
) ([]LoggedEvent, bool, int64) {
	jid, err := uuid.Parse(jobID)
	if err != nil {
		return nil, false, afterSeq
	}
	rows, err := s.eventLog.ListAfterSeq(ctx, jid, afterSeq)
	if err != nil {
		return nil, false, afterSeq
	}
	var out []LoggedEvent
	maxSeq := afterSeq
	for _, row := range rows {
		le := LoggedEvent{
			JobID: jobID,
			Seq:   row.Seq,
			Event: json.RawMessage(row.Event),
		}
		if allowedForMode(mode, row.EventType) {
			out = append(out, le)
			maxSeq = row.Seq
		}
	}

	terminal := status == "succeeded" || status == "failed" || status == "cancelled"
	if terminal {
		userErr := errMsg
		if userErr != "" {
			userErr = FormatUserRunError(userErr)
		}
		finSeq := maxSeq + 1
		if afterSeq < finSeq {
			finished, _ := EncodeLoggedEvent(jobID, finSeq, JobRunFinished(jobID, JobRunFinishedPayload{
				Status: status, Output: output, Error: userErr,
			}))
			if allowedForMode(mode, "job_run_finished") {
				out = append(out, finished)
				maxSeq = finSeq
			}
		}
		return out, true, maxSeq
	}

	if len(rows) == 0 && afterSeq < 1 {
		now := time.Now().UnixMilli()
		started := LoggedEvent{
			JobID: jobID, Seq: 1, Timestamp: now,
			Event: json.RawMessage(`{"type":"RUN_STARTED","threadId":"` + jobID + `","runId":"` + jobID + `"}`),
		}
		if allowedForMode(mode, EventRUNStarted) {
			out = append(out, started)
			maxSeq = 1
		}
	}
	return out, false, maxSeq
}

func allowedForMode(mode Mode, eventType string) bool {
	if mode == ModeChat {
		return AllowedInChat(eventType)
	}
	return true
}
