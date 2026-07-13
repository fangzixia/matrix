package stream

import (
	"context"

	agui "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

type ScopeSink struct {
	Inner           Sink
	AgentID         string
	ParentAgentID   string
	ParentToolUseID string
}

func (s ScopeSink) Publish(ctx context.Context, ev Event) error {
	if s.Inner == nil {
		return nil
	}
	return s.Inner.Publish(ctx, s.tag(ev))
}

func (s ScopeSink) tag(ev Event) Event {
	if s.AgentID == "" || ev == nil {
		return ev
	}
	switch e := ev.(type) {
	case *agui.TextMessageStartEvent:
		if e.Name != "" {
			return ev
		}
		opts := []agui.TextMessageStartOption{agui.WithRole("assistant"), agui.WithName(s.AgentID)}
		return agui.NewTextMessageStartEvent(e.MessageID, opts...)
	case *agui.TextMessageChunkEvent:
		if e.Name != nil && *e.Name != "" {
			return ev
		}
		return agui.NewTextMessageChunkEvent(e.MessageID, e.Role, e.Delta).WithChunkName(s.AgentID)
	}
	return ev
}
