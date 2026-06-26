package view

import (
	"context"
	"time"
)

// CatchUpAfterSeq 从 DB 生成 seq > afterSeq 的 SSE 事件；Run 终态时返回 done=true。
// SSE 流的唯一数据源：不依赖进程内 Hub，支持 Worker 与 HTTP 分离部署。
func (s *Store) CatchUpAfterSeq(
	ctx context.Context,
	runID string,
	mode Mode,
	afterSeq int64,
	status, output, errMsg, mergeStatus string,
) ([]Envelope, bool, int64) {
	now := time.Now().UnixMilli()
	var out []Envelope
	maxSeq := afterSeq
	terminal := status == "succeeded" || status == "failed" || status == "cancelled"

	snap, _ := s.Snapshot(ctx, runID)
	if snap != nil && snap.Seq > afterSeq {
		env := Envelope{
			Type:      EventSTATESnapshot,
			RunID:     runID,
			Seq:       snap.Seq,
			Timestamp: now,
			Payload:   *snap,
		}
		if allowedForMode(mode, env.Type) {
			out = append(out, env)
			maxSeq = snap.Seq
		}
	}

	if terminal {
		finSeq := int64(2)
		if snap != nil {
			finSeq = snap.Seq + 1
		}
		if afterSeq < finSeq {
			finished := Envelope{
				Type:      EventRUNFinished,
				RunID:     runID,
				Seq:       finSeq,
				Timestamp: now,
				Payload: RunFinishedPayload{
					Status: status, Output: output, Error: errMsg, MergeStatus: mergeStatus,
				},
			}
			if allowedForMode(mode, finished.Type) {
				out = append(out, finished)
				maxSeq = finSeq
			}
		}
		return out, true, maxSeq
	}

	if snap == nil {
		if afterSeq < 1 {
			label := "Agent 正在工作…"
			if status == "queued" || status == "pending" {
				label = "任务排队中…"
			}
			started := Envelope{
				Type:      EventRUNStarted,
				RunID:     runID,
				Seq:       1,
				Timestamp: now,
				Payload:   RunStartedPayload{StatusLabel: label},
			}
			if allowedForMode(mode, started.Type) {
				out = append(out, started)
				maxSeq = 1
			}
		} else if afterSeq < 2 && status == "running" {
			env := Envelope{
				Type:      EventACTIVITYSnapshot,
				RunID:     runID,
				Seq:       2,
				Timestamp: now,
				Payload:   ActivitySnapshotPayload{StatusLabel: "Agent 正在工作…"},
			}
			if allowedForMode(mode, env.Type) {
				out = append(out, env)
				maxSeq = 2
			}
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
