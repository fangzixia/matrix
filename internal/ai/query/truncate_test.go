package query

import (
	"strings"
	"testing"
)

func TestTruncateRunesUTF8(t *testing.T) {
	s := "你好世界Hello"
	if got := TruncateRunes(s, 4); !strings.HasPrefix(got, "你好世界") {
		t.Fatalf("expected first 4 runes preserved, got %q", got)
	}
	if got := TruncateRunes("abc", 10); got != "abc" {
		t.Fatalf("short string unchanged: got %q", got)
	}
}
