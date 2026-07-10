package repo

import (
	"context"
	"time"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
)

// ArtifactStore 封装评测产物索引持久化。
type ArtifactStore struct {
	c *catalog
}

func newArtifactStore(c *catalog) *ArtifactStore { return &ArtifactStore{c: c} }

func (s *ArtifactStore) ListEvaluations(ctx context.Context, projectID uuid.UUID) ([]models.Artifact, error) {
	return s.c.artifact.ListByProjectAndKind(ctx, projectID, "evaluation")
}

// IndexAfterRunParams Run 成功后索引评测产物参数。
type ArtifactIndexParams struct {
	ProjectID    uuid.UUID
	RepositoryID *uuid.UUID
	RunID        uuid.UUID
	Path         string
	PlanPath     string
	Title        string
}

func (s *ArtifactStore) IndexAfterRun(ctx context.Context, p ArtifactIndexParams) error {
	row := models.Artifact{
		ProjectID: p.ProjectID, RepositoryID: p.RepositoryID, RunID: &p.RunID,
		Kind: "evaluation", Path: p.Path, PlanPath: p.PlanPath, Title: p.Title,
		CreatedAt: time.Now(),
	}
	return s.c.artifact.UpsertAfterRun(ctx, &row)
}
