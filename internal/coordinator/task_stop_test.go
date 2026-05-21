package coordinator

import (
	"context"
	"strings"
	"testing"
	"time"

	"matrix/internal/agent"
	"matrix/internal/query"
)

func TestTaskStop_cancelsTrackedWorker(t *testing.T) {
	reg := agent.NewRegistry()
	rc := NewRunControl()
	rc.SetParent(context.Background())

	id := agent.NewID()
	reg.Register(&agent.Record{
		ID:        id,
		Status:    agent.StatusRunning,
		CreatedAt: time.Now(),
	})

	workerCtx, end := rc.Begin(id)
	defer end()

	finished := make(chan query.Result, 1)
	go func() {
		<-workerCtx.Done()
		finished <- query.Result{
			StopReason: query.StopAborted,
			Err:        workerCtx.Err(),
		}
	}()

	tool := NewTaskStopTool(Config{AgentRegistry: reg, RunControl: rc})
	out, err := tool.Execute(context.Background(), map[string]any{
		"task_id": string(id),
		"reason":  "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "已停止") {
		t.Fatalf("unexpected task_stop output: %q", out)
	}

	select {
	case r := <-finished:
		if r.StopReason != query.StopAborted {
			t.Fatalf("stop reason = %s, want aborted", r.StopReason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker goroutine not cancelled")
	}

	rec := reg.Get(id)
	if rec == nil || rec.Status != agent.StatusStopped {
		t.Fatalf("registry status = %v, want stopped", rec)
	}
}
