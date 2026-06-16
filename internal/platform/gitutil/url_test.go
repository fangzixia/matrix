package gitutil

import "testing"

func TestHostFromURL(t *testing.T) {
	cases := map[string]string{
		"git@github.com:org/repo.git":        "github.com",
		"git@gitlab.example.com:group/p.git": "gitlab.example.com",
		"https://github.com/org/repo.git":    "github.com",
		"ssh://git@gitlab.com/group/repo":    "gitlab.com",
	}
	for url, want := range cases {
		if got := HostFromURL(url); got != want {
			t.Fatalf("%s: got %q want %q", url, got, want)
		}
	}
}

func TestMatchHost(t *testing.T) {
	if !MatchHost("github.com", "*") {
		t.Fatal("expected * match")
	}
	if !MatchHost("gitlab.corp.com", "corp.com") {
		t.Fatal("expected suffix match")
	}
	if MatchHost("github.com", "gitlab.com") {
		t.Fatal("expected no match")
	}
}
