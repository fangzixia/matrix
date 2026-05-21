package coordinator

import (
	"context"
	"testing"
	"time"

	"matrix/internal/agent"
)

func TestRunControl_StopCancelsContext(t *testing.T) {
	rc := NewRunControl()
	rc.SetParent(context.Background())
	id := agent.ID("agent-test")

	ctx, cleanup := rc.Begin(id)
	defer cleanup()

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	if !rc.Stop(id) {
		t.Fatal("Stop returned false")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker context not cancelled")
	}
}

func TestRunControl_StopNotRunning(t *testing.T) {
	rc := NewRunControl()
	if rc.Stop(agent.ID("missing")) {
		t.Fatal("expected false for unknown id")
	}
}

func TestRunControl_SetParentCancelsWorkers(t *testing.T) {
	rc := NewRunControl()
	parent, cancelParent := context.WithCancel(context.Background())
	rc.SetParent(parent)

	id := agent.ID("agent-parent")
	ctx, cleanup := rc.Begin(id)
	defer cleanup()

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	cancelParent()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker not cancelled when parent session cancelled")
	}
}
