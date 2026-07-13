package stream

import (
	"context"

	agui "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// Event is a standard AG-UI protocol event.
type Event = agui.Event

// Sink receives AG-UI session events; Publish should block until consumed or ctx is cancelled.
type Sink interface {
	Publish(ctx context.Context, ev Event) error
}

// NopSink discards all events (non-streaming runs).
type NopSink struct{}

// Publish is a no-op.
func (NopSink) Publish(context.Context, Event) error { return nil }

// FuncSink implements Sink with a function.
type FuncSink func(ctx context.Context, ev Event) error

// Publish invokes the underlying function.
func (f FuncSink) Publish(ctx context.Context, ev Event) error {
	if f == nil {
		return nil
	}
	return f(ctx, ev)
}

// ChanSink writes events to a channel (blocking send).
type ChanSink struct {
	Ch chan<- Event
}

// Publish sends ev to Ch; returns ctx.Err() on cancellation.
func (s ChanSink) Publish(ctx context.Context, ev Event) error {
	if s.Ch == nil {
		return nil
	}
	select {
	case s.Ch <- ev:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
