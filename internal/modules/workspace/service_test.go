package workspace

import (
	"context"
	"path/filepath"
	"testing"

	"matrix/internal/platform/config"
	"matrix/internal/platform/storage"

	"github.com/google/uuid"
)

type staticProjectKey string

func (s staticProjectKey) ProjectWorkspaceKey(context.Context, uuid.UUID) (string, error) {
	return string(s), nil
}

func TestResolveForRejectsSiblingWithSamePrefix(t *testing.T) {
	root := t.TempDir()
	svc := NewService(storage.Paths{WorkspacesDir: root}, config.GitConfig{}, nil)
	svc.SetProjectKeyResolver(staticProjectKey("proj"))
	projectID := uuid.New()

	escaped := filepath.Join("..", "repo-evil", "file.txt")
	if _, err := svc.resolveFor(projectID, "", escaped); err == nil {
		t.Fatal("expected path outside repository root to be rejected")
	}
}
