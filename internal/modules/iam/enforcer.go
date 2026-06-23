// Package iam 项目级 RBAC：Guest → Owner 五级角色与权限校验。
package iam

import (
	"context"
	"errors"
	"matrix/internal/platform/db/models"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Role 是项目成员角色。
type Role string

const (
	// RoleGuest 是最低只读角色。
	RoleGuest Role = "guest"
	// RoleReporter 可拉取 Git 但不能创建 Run。
	RoleReporter Role = "reporter"
	// RoleDeveloper 可创建 Run 与拉取 Git。
	RoleDeveloper Role = "developer"
	// RoleMaintainer 可管理成员与推送 Git。
	RoleMaintainer Role = "maintainer"
	// RoleOwner 拥有项目全部权限。
	RoleOwner Role = "owner"
)

var roleLevel = map[Role]int{
	RoleGuest: 10, RoleReporter: 20, RoleDeveloper: 30,
	RoleMaintainer: 40, RoleOwner: 50,
}

// RoleAtLeast 判断 have 是否不低于 need 的角色等级。
func RoleAtLeast(have Role, need Role) bool {
	return roleLevel[have] >= roleLevel[need]
}

// ValidateRole 校验角色字符串是否为已知五级角色之一。
func ValidateRole(role Role) error {
	if _, ok := roleLevel[role]; !ok {
		return errors.New("无效的角色")
	}
	return nil
}

// Enforcer 校验用户对项目的有效角色与访问权限。
type Enforcer struct {
	db *gorm.DB
}

// NewEnforcer 创建 Enforcer。
func NewEnforcer(db *gorm.DB) *Enforcer {
	return &Enforcer{db: db}
}

// EffectiveRole 计算用户在项目中的有效角色。
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

// GroupInheritedRole 返回用户通过组继承的项目角色。
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

// GroupRole 返回用户在指定组中的角色。
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

// CanAccessGroup 校验用户是否满足组级最低角色要求。
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

// ProjectVisibility 返回项目的可见性设置。
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

// CanAccess 校验用户是否满足项目最低角色要求（含组继承的有效角色）。
func (e *Enforcer) CanAccess(ctx context.Context, userID, projectID uuid.UUID, min Role, isAdmin bool) (bool, error) {
	if isAdmin {
		return true, nil
	}
	role, isMember, err := e.EffectiveRole(ctx, userID, projectID)
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

// ProjectRole 返回用户在项目中的直接成员角色。
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

// CanProject 校验当前会话用户是否满足项目权限。
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

// roleForAction 将 IAM 动作映射为最低所需角色。
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

// MemberService 管理项目成员的增删改查。
type MemberService struct {
	db *gorm.DB
}

// NewMemberService 创建 MemberService。
func NewMemberService(db *gorm.DB) *MemberService {
	return &MemberService{db: db}
}

// MemberDTO 是项目成员列表项。
type MemberDTO struct {
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// List 返回列表。
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

// Add 执行对应操作。
func (s *MemberService) Add(ctx context.Context, projectID, userID uuid.UUID, role Role) error {
	if err := ValidateRole(role); err != nil {
		return err
	}
	m := models.ProjectMember{ProjectID: projectID, UserID: userID, Role: string(role)}
	return s.db.WithContext(ctx).Save(&m).Error
}

// UpdateRole 更新记录。
func (s *MemberService) UpdateRole(ctx context.Context, projectID, userID uuid.UUID, role Role) error {
	if err := ValidateRole(role); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&models.ProjectMember{}).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Update("role", string(role)).Error
}

// Remove 删除记录。
func (s *MemberService) Remove(ctx context.Context, projectID, userID uuid.UUID) error {
	return s.db.WithContext(ctx).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Delete(&models.ProjectMember{}).Error
}
