package run

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"matrix/internal/platform/db/models"
)

type stubHarnessWorkspace struct {
	docsRoot string
}

func (s *stubHarnessWorkspace) ProjectWorkspaceKey(context.Context, uuid.UUID) (string, error) {
	return "test", nil
}
func (s *stubHarnessWorkspace) CreateRunRepo(context.Context, uuid.UUID, *uuid.UUID, uuid.UUID) (string, error) {
	return "", nil
}
func (s *stubHarnessWorkspace) CopyRepo(context.Context, uuid.UUID, string, uuid.UUID) (string, error) {
	return "", nil
}
func (s *stubHarnessWorkspace) RemoveRunRepo(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (s *stubHarnessWorkspace) DocsRoot(context.Context, uuid.UUID) (string, error) {
	return s.docsRoot, nil
}
func (s *stubHarnessWorkspace) ResolveDocPath(_ uuid.UUID, logical string) (string, error) {
	return filepath.Join(s.docsRoot, filepath.FromSlash(strings.TrimPrefix(logical, "docs/"))), nil
}
func (s *stubHarnessWorkspace) SanitizeDocLogicalPath(logical string) (string, error) {
	return logical, nil
}
func (s *stubHarnessWorkspace) DocSandboxDir(context.Context, uuid.UUID) (string, error) {
	return s.docsRoot, nil
}
func (s *stubHarnessWorkspace) MatrixDir(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", nil
}

func TestHarnessEvalAbsPath_implementAutoLookup(t *testing.T) {
	docsRoot := t.TempDir()
	plansDir := filepath.Join(docsRoot, "plans")
	evalsDir := filepath.Join(docsRoot, "evaluations")
	for _, d := range []string{plansDir, evalsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	planRel := "docs/plans/PLAN-foo.md"
	if err := os.WriteFile(filepath.Join(docsRoot, "plans", "PLAN-foo.md"), []byte("# plan"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsRoot, "evaluations", "EVAL-PLAN-foo-latest.md"), []byte("# eval"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Service{workspace: &stubHarnessWorkspace{docsRoot: docsRoot}}
	m := &models.Run{FilePath: planRel}
	got := s.harnessEvalAbsPath(uuid.New(), docsRoot, m, "implement")
	want := filepath.Join(docsRoot, "evaluations", "EVAL-PLAN-foo-latest.md")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestHarnessEvalAbsPath_implementNoEval(t *testing.T) {
	docsRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(docsRoot, "evaluations"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := &Service{workspace: &stubHarnessWorkspace{docsRoot: docsRoot}}
	m := &models.Run{FilePath: "docs/plans/PLAN-missing.md"}
	got := s.harnessEvalAbsPath(uuid.New(), docsRoot, m, "implement")
	if got != "" {
		t.Fatalf("expected empty eval path, got %q", got)
	}
}

func TestHarnessEvalAbsPath_buildUsesEvalFilePath(t *testing.T) {
	docsRoot := t.TempDir()
	evalsDir := filepath.Join(docsRoot, "evaluations")
	if err := os.MkdirAll(evalsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	evalRel := "docs/evaluations/EVAL-PLAN-bar.md"
	if err := os.WriteFile(filepath.Join(evalsDir, "EVAL-PLAN-bar.md"), []byte("# eval"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Service{workspace: &stubHarnessWorkspace{docsRoot: docsRoot}}
	m := &models.Run{EvalFilePath: evalRel}
	got := s.harnessEvalAbsPath(uuid.New(), docsRoot, m, "build")
	want := filepath.Join(docsRoot, "evaluations", "EVAL-PLAN-bar.md")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildHarnessMessages_implementWithEval(t *testing.T) {
	evalAbs := `E:\docs\evaluations\EVAL-PLAN-foo.md`
	msgs := BuildHarnessMessages("implement", "task", "docs/plans/PLAN-foo.md",
		`E:\docs\plans\PLAN-foo.md`, evalAbs, `E:\repo`, `E:\docs`, "22222222-2222-2222-2222-222222222222")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	content := msgs[0].Content
	if !strings.Contains(content, evalAbs) {
		t.Fatalf("expected eval in prompt: %s", content)
	}
	if !strings.Contains(content, "实现 Run（代码复制来源）") {
		t.Fatalf("expected source run id in prefix: %s", content)
	}
}
