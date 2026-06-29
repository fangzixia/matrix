package tools

import (
	"path/filepath"
	"testing"
)

func TestGrepPathLineSeparator(t *testing.T) {
	tests := []struct {
		line string
		want int
	}{
		{`E:\matrix\repo\internal\ai\ports\runtime.go:30:	AllowShell bool`, 43},
		{`/home/user/project/main.go:42:func main()`, 26},
		{`relative.go:10:content`, 11},
		{`no-colon-line`, -1},
		{`E:`, -1},
	}
	for _, tt := range tests {
		got := grepPathLineSeparator(tt.line)
		if tt.want >= 0 {
			if got != tt.want {
				t.Errorf("grepPathLineSeparator(%q) = %d, want %d", tt.line, got, tt.want)
				continue
			}
			if got >= len(tt.line) || tt.line[got] != ':' {
				t.Errorf("grepPathLineSeparator(%q) separator not at ':'", tt.line)
			}
		} else if got >= 0 {
			t.Errorf("grepPathLineSeparator(%q) = %d, want -1", tt.line, got)
		}
	}
}

func TestGrepSplitPathAndRest(t *testing.T) {
	file, rest, ok := grepSplitPathAndRest(`E:\repo\file.go:30:match`)
	if !ok {
		t.Fatal("expected ok")
	}
	if file != `E:\repo\file.go` || rest != `:30:match` {
		t.Fatalf("got file=%q rest=%q", file, rest)
	}

	file, rest, ok = grepSplitPathAndRest(`/tmp/file.go:5:match`)
	if !ok || file != `/tmp/file.go` || rest != `:5:match` {
		t.Fatalf("unix: file=%q rest=%q ok=%v", file, rest, ok)
	}
}

func TestGrepAbsolutizePathsWindowsContent(t *testing.T) {
	searchRoot := `E:\matrix\data\workspaces\matrix\repo`
	lines := []string{
		`E:\matrix\data\workspaces\matrix\repo\internal\ai\ports\runtime.go:30:	AllowShell bool`,
		`internal\modules\settings\service.go:43:	AllowShell bool`,
	}
	got := grepAbsolutizePaths(lines, searchRoot, "content")

	wantAbs := filepath.Clean(`E:\matrix\data\workspaces\matrix\repo\internal\ai\ports\runtime.go`)
	if got[0] != wantAbs+`:30:	AllowShell bool` {
		t.Errorf("absolute line:\n  got  %q\n  want %q", got[0], wantAbs+`:30:	AllowShell bool`)
	}

	wantRel := filepath.Clean(filepath.Join(searchRoot, `internal\modules\settings\service.go`))
	if got[1] != wantRel+`:43:	AllowShell bool` {
		t.Errorf("relative line:\n  got  %q\n  want %q", got[1], wantRel+`:43:	AllowShell bool`)
	}

	for _, line := range got {
		if count := countSubstring(line, searchRoot); count > 1 {
			t.Errorf("duplicate searchRoot in %q", line)
		}
	}
}

func TestGrepAbsolutizePathsCountMode(t *testing.T) {
	searchRoot := `E:\matrix\repo`
	lines := []string{`E:\matrix\repo\main.go:3`}
	got := grepAbsolutizePaths(lines, searchRoot, "count")
	want := filepath.Clean(`E:\matrix\repo\main.go`) + `:3`
	if got[0] != want {
		t.Errorf("got %q, want %q", got[0], want)
	}
}

func TestGrepAbsolutizePathsFilesWithMatches(t *testing.T) {
	searchRoot := `E:\matrix\repo`
	lines := []string{
		`E:\matrix\repo\main.go`,
		`pkg\util.go`,
	}
	got := grepAbsolutizePaths(lines, searchRoot, "files_with_matches")
	if got[0] != filepath.Clean(`E:\matrix\repo\main.go`) {
		t.Errorf("abs file: got %q", got[0])
	}
	if got[1] != filepath.Clean(filepath.Join(searchRoot, `pkg\util.go`)) {
		t.Errorf("rel file: got %q", got[1])
	}
}

func countSubstring(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}
