package repo

import (
	"context"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
)

// ProjectStore 封装项目与成员持久化。
type ProjectStore struct {
	c *catalog
}

func newProjectStore(c *catalog) *ProjectStore { return &ProjectStore{c: c} }

// CreateProjectParams 创建项目参数。
type CreateProjectParams struct {
	Name, Path, GitURL, GitBranch, Visibility string
	GroupID                                   *uuid.UUID
	OwnerID                                   uuid.UUID
	OwnerRole                                 string
}

// Create 原子创建项目及 owner 成员。
func (s *ProjectStore) Create(ctx context.Context, p CreateProjectParams) (*models.Project, error) {
	m := models.Project{
		Name: p.Name, Path: p.Path, GitURL: p.GitURL, GitBranch: p.GitBranch,
		Visibility: p.Visibility, GroupID: p.GroupID, OwnerID: p.OwnerID,
	}
	if err := s.c.project.CreateWithOwner(ctx, &m, p.OwnerID, p.OwnerRole); err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *ProjectStore) GetByID(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	return s.c.project.GetByID(ctx, id)
}

func (s *ProjectStore) ExistsByPath(ctx context.Context, path string, excludeID uuid.UUID) (bool, error) {
	return s.c.project.ExistsByPath(ctx, path, excludeID)
}

func (s *ProjectStore) Save(ctx context.Context, m *models.Project) error {
	return s.c.project.Save(ctx, m)
}

func (s *ProjectStore) Delete(ctx context.Context, id uuid.UUID) error {
	return s.c.project.Delete(ctx, id)
}

func (s *ProjectStore) ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool, scope, visInternal, visPublic string) ([]models.Project, error) {
	return s.c.project.ListForUser(ctx, userID, isAdmin, scope, visInternal, visPublic)
}

func (s *ProjectStore) ListAll(ctx context.Context) ([]models.Project, error) {
	return s.c.project.ListAll(ctx)
}
