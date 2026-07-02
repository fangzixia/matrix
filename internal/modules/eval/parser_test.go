package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLatestEval_matchesPlanBase(t *testing.T) {
	docsRoot := t.TempDir()
	evalsDir := filepath.Join(docsRoot, "evaluations")
	if err := os.MkdirAll(evalsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	older := filepath.Join(evalsDir, "EVAL-PLAN-foo-round1.md")
	newer := filepath.Join(evalsDir, "EVAL-PLAN-foo-round2.md")
	unrelated := filepath.Join(evalsDir, "EVAL-PLAN-bar.md")
	for _, p := range []string{older, newer, unrelated} {
		if err := os.WriteFile(p, []byte("# eval\n综合分: 7.0"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	olderTime := time.Now().Add(-2 * time.Hour)
	newerTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(older, olderTime, olderTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, newerTime, newerTime); err != nil {
		t.Fatal(err)
	}

	rel, content, err := LatestEval(docsRoot, "docs/plans/PLAN-foo.md")
	if err != nil {
		t.Fatal(err)
	}
	if rel != "docs/evaluations/EVAL-PLAN-foo-round2.md" {
		t.Fatalf("got rel %q, want latest PLAN-foo eval", rel)
	}
	if !strings.Contains(content, "综合分") {
		t.Fatalf("unexpected content: %q", content)
	}
}
