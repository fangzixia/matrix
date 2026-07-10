// Package group 用户组成员与项目权限继承。
package group

import (
	"context"
	"errors"
	"matrix/internal/modules/iam"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/db/repo"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GroupDTO 是用户组 API 返回的数据传输对象。
type GroupDTO struct {
	ID              uuid.UUID             `json:"id"`
	Name            string                `json:"name"`
	OwnerID         uuid.UUID             `json:"owner_id"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	CurrentUserRole *iam.Role             `json:"current_user_role,omitempty"`
	Permissions     *iam.GroupPermissions `json:"permissions,omitempty"`
}

// MemberDTO 是用户组成员 API 返回的数据传输对象。
type MemberDTO struct {
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      iam.Role  `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateInput 是创建用户组时的请求参数。
type CreateInput struct {
	Name string `json:"name"`
}

// Service 管理用户组成员与项目权限继承。
type Service struct {
	stores *repo.Stores
}

// NewService 创建用户组服务实例。
func NewService(stores *repo.Stores) *Service {
	return &Service{stores: stores}
}

// ListForUser 返回当前用户可见的列表。
func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]GroupDTO, error) {
	rows, err := s.stores.Group.ListForUser(ctx, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	out := make([]GroupDTO, len(rows))
	for i := range rows {
		out[i] = *s.enrich(ctx, &rows[i], userID, isAdmin)
	}
	return out, nil
}

// Get 执行对应操作。
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*GroupDTO, error) {
	m, err := s.stores.Group.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toDTO(m), nil
}

// GetForUser 按用户权限返回单条记录（含当前用户角色与权限）。
func (s *Service) GetForUser(ctx context.Context, id, userID uuid.UUID, isAdmin bool) (*GroupDTO, error) {
	m, err := s.stores.Group.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.enrich(ctx, m, userID, isAdmin), nil
}

// Create 创建记录。
func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, in CreateInput) (*GroupDTO, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errors.New("组名称不能为空")
	}
	m, err := s.stores.Group.Create(ctx, repo.CreateGroupParams{
		Name: name, OwnerID: ownerID, OwnerRole: string(iam.RoleOwner),
	})
	if err != nil {
		return nil, err
	}
	return s.enrich(ctx, m, ownerID, false), nil
}

// Update 更新记录。
func (s *Service) Update(ctx context.Context, id uuid.UUID, name *string) (*GroupDTO, error) {
	m, err := s.stores.Group.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return nil, errors.New("组名称不能为空")
		}
		m.Name = trimmed
	}
	if err := s.stores.Group.Save(ctx, m); err != nil {
		return nil, err
	}
	return toDTO(m), nil
}

// Delete 删除记录。
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.stores.Group.Delete(ctx, id)
}

// ListMembers 返回组成员列表。
func (s *Service) ListMembers(ctx context.Context, groupID uuid.UUID) ([]MemberDTO, error) {
	rows, err := s.stores.Group.ListMembers(ctx, groupID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberDTO, 0, len(rows))
	for _, r := range rows {
		u, err := s.stores.User.GetByID(ctx, r.UserID)
		if err != nil {
			continue
		}
		out = append(out, MemberDTO{
			UserID: r.UserID, Username: u.Username, Email: u.Email,
			Name: u.Name, Role: iam.Role(r.Role), CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

// AddMember 向组添加成员。
func (s *Service) AddMember(ctx context.Context, groupID, userID uuid.UUID, role iam.Role) error {
	if err := iam.ValidateRole(role); err != nil {
		return err
	}
	return s.stores.Group.AddMember(ctx, groupID, userID, string(role))
}

// UpdateMember 更新组成员角色。
func (s *Service) UpdateMember(ctx context.Context, groupID, userID uuid.UUID, role iam.Role) error {
	if err := iam.ValidateRole(role); err != nil {
		return err
	}
	return s.stores.Group.UpdateMemberRole(ctx, groupID, userID, string(role))
}

// RemoveMember 从组移除成员。
func (s *Service) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	return s.stores.Group.RemoveMember(ctx, groupID, userID)
}

// enrich 为实体补充当前用户权限等扩展字段。
func (s *Service) enrich(ctx context.Context, m *models.Group, userID uuid.UUID, isAdmin bool) *GroupDTO {
	g := toDTO(m)
	enforcer := iam.NewEnforcer(s.stores.IAM)
	if isAdmin {
		g.CurrentUserRole = new(iam.RoleOwner)
		g.Permissions = new(iam.PermissionsForGroupRole(iam.RoleOwner))
		return g
	}
	role, ok, _ := enforcer.GroupRole(ctx, userID, m.ID)
	if ok {
		g.CurrentUserRole = &role
		g.Permissions = new(iam.PermissionsForGroupRole(role))
	}
	return g
}

// toDTO 将数据库模型转换为 API DTO。
func toDTO(m *models.Group) *GroupDTO {
	return &GroupDTO{
		ID: m.ID, Name: m.Name, OwnerID: m.OwnerID,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}
