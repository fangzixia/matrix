package view

import (
	"encoding/json"

	agui "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// LoggedEvent 是 SSE 回放的事件日志条目（扁平 AG-UI JSON + 宿主 seq）。
type LoggedEvent struct {
	JobID     string          `json:"jobId"`
	Seq       int64           `json:"seq"`
	Timestamp int64           `json:"timestamp,omitempty"`
	Event     json.RawMessage `json:"event"`
}

// EncodeLoggedEvent 将 AG-UI 事件序列化为 LoggedEvent。
func EncodeLoggedEvent(jobID string, seq int64, ev agui.Event) (LoggedEvent, error) {
	if ev == nil {
		return LoggedEvent{}, nil
	}
	b, err := ev.ToJSON()
	if err != nil {
		return LoggedEvent{}, err
	}
	var ts int64
	if t := ev.Timestamp(); t != nil {
		ts = *t
	}
	return LoggedEvent{
		JobID:     jobID,
		Seq:       seq,
		Timestamp: ts,
		Event:     b,
	}, nil
}

// DecodeEvent 从 JSON 解析 AG-UI 事件。
func DecodeEvent(data []byte) (agui.Event, error) {
	return agui.EventFromJSON(data)
}
