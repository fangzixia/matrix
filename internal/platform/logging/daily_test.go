package logging

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDailyWriterRotatesByDate(t *testing.T) {
	dir := t.TempDir()
	fixed := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	w, err := newDailyWriter(dir, "system", 7)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	_ = w.Close()
	w.file = nil
	w.currentDate = ""
	w.now = func() time.Time { return fixed }
	if _, err := w.Write([]byte("day1\n")); err != nil {
		t.Fatal(err)
	}
	w.now = func() time.Time { return fixed.Add(24 * time.Hour) }
	if _, err := w.Write([]byte("day2\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "system", "2026-06-29.log")); err != nil {
		t.Fatalf("missing day1 log: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "system", "2026-06-30.log")); err != nil {
		t.Fatalf("missing day2 log: %v", err)
	}
}

func TestArgsToMap(t *testing.T) {
	m := argsToMap("a", 1, "b", "two")
	if m["a"] != 1 || m["b"] != "two" {
		t.Fatalf("unexpected map: %#v", m)
	}
}
