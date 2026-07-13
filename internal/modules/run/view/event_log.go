package view

import (
	"context"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
)

// EventLogPersistence 持久化 Run 视图事件日志。
type EventLogPersistence interface {
	AppendViewEvent(ctx context.Context, row *models.RunViewEvent) error
	ListViewEventsAfterSeq(ctx context.Context, jobID uuid.UUID, afterSeq int64) ([]models.RunViewEvent, error)
}

// EventLog 管理 run_view_events 追加与回放。
type EventLog struct {
	p EventLogPersistence
}

// NewEventLog 创建事件日志存储。
func NewEventLog(p EventLogPersistence) *EventLog {
	return &EventLog{p: p}
}

// Append 写入一条事件日志。
func (l *EventLog) Append(ctx context.Context, jobID uuid.UUID, seq int64, eventType, eventJSON string) error {
	if l == nil || l.p == nil {
		return nil
	}
	return l.p.AppendViewEvent(ctx, &models.RunViewEvent{
		JobID:     jobID,
		Seq:       seq,
		EventType: eventType,
		Event:     eventJSON,
	})
}

// ListAfterSeq 回放 seq > afterSeq 的事件。
func (l *EventLog) ListAfterSeq(ctx context.Context, jobID uuid.UUID, afterSeq int64) ([]models.RunViewEvent, error) {
	if l == nil || l.p == nil {
		return nil, nil
	}
	return l.p.ListViewEventsAfterSeq(ctx, jobID, afterSeq)
}
