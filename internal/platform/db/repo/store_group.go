package repo

import (
	"context"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
)

// GroupStore 封装用户组持久化。
type GroupStore struct {
	c *catalog
}

func newGroupStore(c *catalog) *GroupStore { return &GroupStore{c: c} }

// CreateGroupParams 创建用户组参数。
type CreateGroupParams struct {
	Name      string
	OwnerID   uuid.UUID
	OwnerRole string
}

func (s *GroupStore) Create(ctx context.Context, p CreateGroupParams) (*models.Group, error) {
	m := models.Group{Name: p.Name, OwnerID: p.OwnerID}
	if err := s.c.group.CreateWithOwner(ctx, &m, p.OwnerID, p.OwnerRole); err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *GroupStore) GetByID(ctx context.Context, id uuid.UUID) (*models.Group, error) {
	return s.c.group.GetByID(ctx, id)
}

func (s *GroupStore) Save(ctx context.Context, m *models.Group) error {
	return s.c.group.Save(ctx, m)
}

func (s *GroupStore) Delete(ctx context.Context, id uuid.UUID) error {
	return s.c.group.DeleteWithCleanup(ctx, id)
}

func (s *GroupStore) ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]models.Group, error) {
	return s.c.group.ListForUser(ctx, userID, isAdmin)
}

func (s *GroupStore) ListMembers(ctx context.Context, groupID uuid.UUID) ([]models.GroupMember, error) {
	return s.c.groupMember.ListByGroup(ctx, groupID)
}

func (s *GroupStore) AddMember(ctx context.Context, groupID, userID uuid.UUID, role string) error {
	return s.c.groupMember.Save(ctx, &models.GroupMember{GroupID: groupID, UserID: userID, Role: role})
}

func (s *GroupStore) UpdateMemberRole(ctx context.Context, groupID, userID uuid.UUID, role string) error {
	return s.c.groupMember.UpdateRole(ctx, groupID, userID, role)
}

func (s *GroupStore) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	return s.c.groupMember.Delete(ctx, groupID, userID)
}

func (s *GroupStore) GetMember(ctx context.Context, groupID, userID uuid.UUID) (*models.GroupMember, error) {
	return s.c.groupMember.Get(ctx, groupID, userID)
}
