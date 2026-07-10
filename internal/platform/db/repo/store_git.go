package repo

import (
	"context"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
)

// GitStore 封装项目 Git 仓库绑定持久化。
type GitStore struct {
	c *catalog
}

func newGitStore(c *catalog) *GitStore { return &GitStore{c: c} }

func (s *GitStore) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.ProjectRepository, error) {
	return s.c.projectRepository.ListByProject(ctx, projectID)
}

func (s *GitStore) GetByProject(ctx context.Context, projectID, id uuid.UUID) (*models.ProjectRepository, error) {
	return s.c.projectRepository.GetByProject(ctx, projectID, id)
}

func (s *GitStore) GetDefault(ctx context.Context, projectID uuid.UUID) (*models.ProjectRepository, error) {
	return s.c.projectRepository.GetDefault(ctx, projectID)
}

func (s *GitStore) CreateWithDefault(ctx context.Context, m *models.ProjectRepository, setDefault bool) error {
	return s.c.projectRepository.CreateWithDefault(ctx, m, setDefault)
}

func (s *GitStore) Create(ctx context.Context, m *models.ProjectRepository) error {
	return s.c.projectRepository.Create(ctx, m)
}

func (s *GitStore) Delete(ctx context.Context, m *models.ProjectRepository) error {
	return s.c.projectRepository.Delete(ctx, m)
}

func (s *GitStore) CountByProject(ctx context.Context, projectID uuid.UUID) (int64, error) {
	return s.c.projectRepository.CountByProject(ctx, projectID)
}

func (s *GitStore) ListProjects(ctx context.Context) ([]models.Project, error) {
	return s.c.project.ListAll(ctx)
}
