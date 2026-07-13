package stream

import (
	"context"

	agui "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

type metaKey struct{}

// Meta carries AG-UI session lineage attached to a TAOR run.
type Meta struct {
	ThreadID    string
	RunID       string
	ParentRunID string
}

// WithMeta stores Meta in ctx for downstream emitters and sinks.
func WithMeta(ctx context.Context, m Meta) context.Context {
	return context.WithValue(ctx, metaKey{}, m)
}

// MetaFrom reads Meta from ctx; zero value when absent.
func MetaFrom(ctx context.Context) Meta {
	if v, ok := ctx.Value(metaKey{}).(Meta); ok {
		return v
	}
	return Meta{}
}

// NewRunID returns a unique AG-UI runId.
func NewRunID() string {
	return agui.GenerateRunID()
}
