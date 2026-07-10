package repo

import (
	"context"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
)

// IAMStore 封装 IAM 权限查询与成员管理持久化。
type IAMStore struct {
	c *catalog
}

func newIAMStore(c *catalog) *IAMStore { return &IAMStore{c: c} }

func (s *IAMStore) GetProjectGroupAndOwner(ctx context.Context, projectID uuid.UUID) (groupID *uuid.UUID, ownerID uuid.UUID, err error) {
	return s.c.project.GetGroupAndOwner(ctx, projectID)
}

func (s *IAMStore) GetGroupMember(ctx context.Context, groupID, userID uuid.UUID) (*models.GroupMember, error) {
	return s.c.groupMember.Get(ctx, groupID, userID)
}

func (s *IAMStore) GetGroup(ctx context.Context, groupID uuid.UUID) (*models.Group, error) {
	return s.c.group.GetByID(ctx, groupID)
}

func (s *IAMStore) GetProjectVisibility(ctx context.Context, projectID uuid.UUID) (string, error) {
	return s.c.project.GetVisibility(ctx, projectID)
}

func (s *IAMStore) GetProjectMember(ctx context.Context, projectID, userID uuid.UUID) (*models.ProjectMember, error) {
	return s.c.projectMember.Get(ctx, projectID, userID)
}

func (s *IAMStore) GetProject(ctx context.Context, projectID uuid.UUID) (*models.Project, error) {
	return s.c.project.GetByID(ctx, projectID)
}

func (s *IAMStore) GetUser(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	return s.c.user.GetByID(ctx, userID)
}

func (s *IAMStore) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	u, err := s.c.user.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return u.IsAdmin, nil
}

func (s *IAMStore) ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]models.ProjectMember, error) {
	return s.c.projectMember.ListByProject(ctx, projectID)
}

func (s *IAMStore) AddProjectMember(ctx context.Context, projectID, userID uuid.UUID, role string) error {
	return s.c.projectMember.Save(ctx, &models.ProjectMember{ProjectID: projectID, UserID: userID, Role: role})
}

func (s *IAMStore) UpdateProjectMemberRole(ctx context.Context, projectID, userID uuid.UUID, role string) error {
	return s.c.projectMember.UpdateRole(ctx, projectID, userID, role)
}

func (s *IAMStore) RemoveProjectMember(ctx context.Context, projectID, userID uuid.UUID) error {
	return s.c.projectMember.Delete(ctx, projectID, userID)
}

// MemberNotFound 判断是否为成员不存在。
func (s *IAMStore) MemberNotFound(err error) bool { return IsNotFound(err) }
