package iam

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"matrix/internal/platform/db/models"
)

type Role string

const (
	RoleGuest      Role = "guest"
	RoleReporter   Role = "reporter"
	RoleDeveloper  Role = "developer"
	RoleMaintainer Role = "maintainer"
	RoleOwner      Role = "owner"
)

var roleLevel = map[Role]int{
	RoleGuest: 10, RoleReporter: 20, RoleDeveloper: 30,
	RoleMaintainer: 40, RoleOwner: 50,
}

func RoleAtLeast(have Role, need Role) bool {
	return roleLevel[have] >= roleLevel[need]
}

type Enforcer struct {
	db *gorm.DB
}

func NewEnforcer(db *gorm.DB) *Enforcer {
	return &Enforcer{db: db}
}

func (e *Enforcer) EffectiveRole(ctx context.Context, userID, projectID uuid.UUID) (Role, bool, error) {
	direct, isMember, err := e.ProjectRole(ctx, userID, projectID)
	if err != nil {
		return "", false, err
	}
	groupRole, hasGroup, err := e.GroupInheritedRole(ctx, userID, projectID)
	if err != nil {
		return "", false, err
	}
	if !isMember && !hasGroup {
		return "", false, nil
	}
	if !isMember {
		return groupRole, true, nil
	}
	if !hasGroup {
		return direct, true, nil
	}
	if roleLevel[groupRole] > roleLevel[direct] {
		return groupRole, true, nil
	}
	return direct, true, nil
}

func (e *Enforcer) GroupInheritedRole(ctx context.Context, userID, projectID uuid.UUID) (Role, bool, error) {
	var p models.Project
	if err := e.db.WithContext(ctx).Select("group_id", "owner_id").First(&p, "id = ?", projectID).Error; err != nil {
		return "", false, err
	}
	if p.GroupID == nil {
		return "", false, nil
	}
	var m models.GroupMember
	err := e.db.WithContext(ctx).Where("group_id = ? AND user_id = ?", *p.GroupID, userID).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		if p.OwnerID == userID {
			return RoleOwner, true, nil
		}
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return Role(m.Role), true, nil
}

func (e *Enforcer) GroupRole(ctx context.Context, userID, groupID uuid.UUID) (Role, bool, error) {
	var m models.GroupMember
	err := e.db.WithContext(ctx).Where("group_id = ? AND user_id = ?", groupID, userID).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		var g models.Group
		if err2 := e.db.WithContext(ctx).First(&g, "id = ?", groupID).Error; err2 == nil && g.OwnerID == userID {
			return RoleOwner, true, nil
		}
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return Role(m.Role), true, nil
}

func (e *Enforcer) CanAccessGroup(ctx context.Context, userID, groupID uuid.UUID, min Role, isAdmin bool) (bool, error) {
	if isAdmin {
		return true, nil
	}
	role, ok, err := e.GroupRole(ctx, userID, groupID)
	if err != nil || !ok {
		return false, err
	}
	return RoleAtLeast(role, min), nil
}

func (e *Enforcer) ProjectVisibility(ctx context.Context, projectID uuid.UUID) (string, error) {
	var p models.Project
	if err := e.db.WithContext(ctx).Select("visibility").First(&p, "id = ?", projectID).Error; err != nil {
		return "", err
	}
	if p.Visibility == "" {
		return "private", nil
	}
	return p.Visibility, nil
}

func (e *Enforcer) CanAccess(ctx context.Context, userID, projectID uuid.UUID, min Role, isAdmin bool) (bool, error) {
	if isAdmin {
		return true, nil
	}
	role, isMember, err := e.ProjectRole(ctx, userID, projectID)
	if err != nil {
		return false, err
	}
	if isMember {
		return RoleAtLeast(role, min), nil
	}
	if min != RoleGuest {
		return false, nil
	}
	vis, err := e.ProjectVisibility(ctx, projectID)
	if err != nil {
		return false, err
	}
	return vis == "internal" || vis == "public", nil
}

func (e *Enforcer) ProjectRole(ctx context.Context, userID, projectID uuid.UUID) (Role, bool, error) {
	var m models.ProjectMember
	err := e.db.WithContext(ctx).Where("project_id = ? AND user_id = ?", projectID, userID).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		var p models.Project
		if err2 := e.db.WithContext(ctx).First(&p, "id = ?", projectID).Error; err2 == nil && p.OwnerID == userID {
			return RoleOwner, true, nil
		}
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return Role(m.Role), true, nil
}

func (e *Enforcer) CanProject(ctx context.Context, userID, projectID uuid.UUID, min Role) (bool, error) {
	return e.CanAccess(ctx, userID, projectID, min, false)
}

// Can 统一授权入口；resourceType: "project"|"admin", action 示例: "project:read", "run:create", "admin:users"
func (e *Enforcer) Can(ctx context.Context, userID uuid.UUID, action, resourceType string, resourceID string) (bool, error) {
	switch resourceType {
	case "admin":
		var u models.User
		if err := e.db.WithContext(ctx).First(&u, "id = ?", userID).Error; err != nil {
			return false, err
		}
		return u.IsAdmin, nil
	case "project":
		pid, err := uuid.Parse(resourceID)
		if err != nil {
			return false, err
		}
		min := roleForAction(action)
		return e.CanProject(ctx, userID, pid, min)
	default:
		return false, nil
	}
}

func roleForAction(action string) Role {
	switch action {
	case "member:manage", "project:settings":
		return RoleMaintainer
	case "repository:pull":
		return RoleReporter
	case "run:create", "run:cancel", "chat:write":
		return RoleDeveloper
	case "project:delete":
		return RoleOwner
	case "project:read", "run:read", "repository:read":
		return RoleGuest
	default:
		return RoleGuest
	}
}

type MemberService struct {
	db *gorm.DB
}

func NewMemberService(db *gorm.DB) *MemberService {
	return &MemberService{db: db}
}

type MemberDTO struct {
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *MemberService) List(ctx context.Context, projectID uuid.UUID) ([]MemberDTO, error) {
	var rows []models.ProjectMember
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]MemberDTO, 0, len(rows))
	for _, r := range rows {
		var u models.User
		if err := s.db.WithContext(ctx).First(&u, "id = ?", r.UserID).Error; err != nil {
			continue
		}
		out = append(out, MemberDTO{
			UserID: r.UserID, Username: u.Username, Email: u.Email,
			Name: u.Name, Role: Role(r.Role), CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *MemberService) Add(ctx context.Context, projectID, userID uuid.UUID, role Role) error {
	m := models.ProjectMember{ProjectID: projectID, UserID: userID, Role: string(role)}
	return s.db.WithContext(ctx).Save(&m).Error
}

func (s *MemberService) UpdateRole(ctx context.Context, projectID, userID uuid.UUID, role Role) error {
	return s.db.WithContext(ctx).Model(&models.ProjectMember{}).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Update("role", string(role)).Error
}

func (s *MemberService) Remove(ctx context.Context, projectID, userID uuid.UUID) error {
	return s.db.WithContext(ctx).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Delete(&models.ProjectMember{}).Error
}
