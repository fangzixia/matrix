package view

import (
	"context"
	"testing"
)

func TestCatchUpAfterSeqTerminalChat(t *testing.T) {
	s := NewStore(nil)
	envs, done, maxSeq := s.CatchUpAfterSeq(
		context.Background(), "r1", ModeChat, 0,
		"succeeded", "hello", "", "",
	)
	if !done {
		t.Fatal("expected terminal")
	}
	if len(envs) != 1 {
		t.Fatalf("expected 1 envelope, got %d", len(envs))
	}
	if envs[0].Type != EventRUNFinished {
		t.Fatalf("got %s", envs[0].Type)
	}
	pl := envs[0].Payload.(RunFinishedPayload)
	if pl.Output != "hello" {
		t.Fatalf("output %q", pl.Output)
	}
	if maxSeq < 1 {
		t.Fatalf("maxSeq %d", maxSeq)
	}
}

func TestCatchUpAfterSeqQueuedRun(t *testing.T) {
	s := NewStore(nil)
	envs, done, maxSeq := s.CatchUpAfterSeq(
		context.Background(), "r1", ModeChat, 0,
		"queued", "", "", "",
	)
	if done {
		t.Fatal("expected not terminal")
	}
	if len(envs) != 1 || envs[0].Type != EventRUNStarted {
		t.Fatalf("got %+v", envs)
	}
	if maxSeq != 1 {
		t.Fatalf("maxSeq %d", maxSeq)
	}
}

func TestCatchUpAfterSeqRunningWithoutSnapshot(t *testing.T) {
	s := NewStore(nil)
	envs, done, maxSeq := s.CatchUpAfterSeq(
		context.Background(), "r1", ModeChat, 1,
		"running", "", "", "",
	)
	if done {
		t.Fatal("expected not terminal")
	}
	if len(envs) != 1 || envs[0].Type != EventACTIVITYSnapshot {
		t.Fatalf("got %+v", envs)
	}
	if maxSeq != 2 {
		t.Fatalf("maxSeq %d", maxSeq)
	}
}

func TestCatchUpAfterSeqIdempotent(t *testing.T) {
	s := NewStore(nil)
	envs, done, maxSeq := s.CatchUpAfterSeq(
		context.Background(), "r1", ModeChat, 2,
		"running", "", "", "",
	)
	if done || len(envs) != 0 {
		t.Fatalf("expected no events, got %d done=%v", len(envs), done)
	}
	if maxSeq != 2 {
		t.Fatalf("maxSeq %d", maxSeq)
	}
}
