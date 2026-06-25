package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutputSpillWriter_AppendAndPath(t *testing.T) {
	dir := t.TempDir()
	ctx := WithSandbox(context.Background(), dir)

	w, err := NewOutputSpillWriter(ctx, "call-abc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(w.Path(), filepath.Join(".matrix", "tool-outputs", "call-abc.log")) {
		t.Fatalf("unexpected path: %s", w.Path())
	}
	if err := w.Append("hello\n"); err != nil {
		t.Fatal(err)
	}
	if err := w.Append("world"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello\nworld" {
		t.Fatalf("got %q", string(data))
	}
}

func TestOutputSpillWriter_TruncatesAtCap(t *testing.T) {
	dir := t.TempDir()
	ctx := WithSandbox(context.Background(), dir)

	w, err := NewOutputSpillWriter(ctx, "big")
	if err != nil {
		t.Fatal(err)
	}
	chunk := strings.Repeat("x", maxToolOutputSpillBytes)
	if err := w.Append(chunk); err != nil {
		t.Fatal(err)
	}
	if err := w.Append("extra"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[output truncated") {
		t.Fatalf("expected truncation marker, got len=%d", len(data))
	}
	if strings.Contains(string(data), "extra") {
		t.Fatal("content after cap should be dropped")
	}
}
