package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestWithMergeAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	old := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(old)

	ctx := With(context.Background(), Fields{
		SessionID: "sess-99",
		Turn:      3,
		Component: "query",
	})
	InfoCtx(ctx, "test message", "extra", "v")
	if !strings.Contains(buf.String(), "sess-99") {
		t.Fatalf("missing session_id: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "test message") {
		t.Fatalf("missing message: %s", buf.String())
	}
}
