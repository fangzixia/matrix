// Package project 项目 CRUD、可见性与路径管理。
package project

import (
	"context"
	"fmt"
	"matrix/internal/modules/iam"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/db/repo"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// VisibilityPrivate 表示仅项目成员可见。
	VisibilityPrivate = "private"
	// VisibilityInternal 表示登录用户可见。
	VisibilityInternal = "internal"
	// VisibilityPublic 表示所有人可见。
	VisibilityPublic = "public"
)

// Project 是项目 API 返回的数据传输对象。
type Project struct {
	ID              uuid.UUID               `json:"id"`
	Name            string                  `json:"name"`
	Path            string                  `json:"path,omitempty"`
	GitURL          string                  `json:"git_url"`
	GitBranch       string                  `json:"git_branch"`
	Visibility      string                  `json:"visibility"`
	GroupID         *uuid.UUID              `json:"group_id,omitempty"`
	OwnerID         uuid.UUID               `json:"owner_id"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
	CurrentUserRole *iam.Role               `json:"current_user_role,omitempty"`
	Permissions     *iam.ProjectPermissions `json:"permissions,omitempty"`
}

// CreateInput 是创建项目时的请求参数。
type CreateInput struct {
	Name       string     `json:"name"`
	Path       string     `json:"path"`
	GitURL     string     `json:"git_url"`
	GitBranch  string     `json:"git_branch"`
	Visibility string     `json:"visibility"`
	GroupID    *uuid.UUID `json:"group_id"`
}

// Service 提供项目 CRUD、可见性与路径管理。
type Service struct {
	stores *repo.Stores
}

// NewService 创建项目服务实例。
func NewService(stores *repo.Stores) *Service {
	return &Service{stores: stores}
}

// Create 创建记录。
func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, in CreateInput) (*Project, error) {
	branch := in.GitBranch
	if branch == "" {
		branch = "main"
	}
	vis := in.Visibility
	if vis == "" {
		vis = VisibilityPrivate
	}
	if strings.TrimSpace(in.Path) == "" {
		return nil, fmt.Errorf("项目编码不能为空")
	}
	code := NormalizeProjectCode(in.Path)
	if code == "" {
		return nil, fmt.Errorf("项目编码不能为空")
	}
	if err := ValidateProjectCode(code); err != nil {
		return nil, err
	}
	exists, err := s.stores.Project.ExistsByPath(ctx, code, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("项目编码 %q 已被使用", code)
	}
	m, err := s.stores.Project.Create(ctx, repo.CreateProjectParams{
		Name: in.Name, Path: code, GitURL: in.GitURL, GitBranch: branch,
		Visibility: vis, GroupID: in.GroupID, OwnerID: ownerID,
		OwnerRole: string(iam.RoleOwner),
	})
	if err != nil {
		return nil, err
	}
	return s.enrich(ctx, m, ownerID, false), nil
}

// Get 执行对应操作。
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Project, error) {
	m, err := s.stores.Project.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toDTO(m), nil
}

// GetForUser 按用户权限返回单条记录。
func (s *Service) GetForUser(ctx context.Context, id, userID uuid.UUID, isAdmin bool) (*Project, error) {
	m, err := s.stores.Project.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.enrich(ctx, m, userID, isAdmin), nil
}

// ListForUser 返回当前用户可见的列表。
func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool, scope string) ([]Project, error) {
	rows, err := s.stores.Project.ListForUser(ctx, userID, isAdmin, scope, VisibilityInternal, VisibilityPublic)
	if err != nil {
		return nil, err
	}
	out := make([]Project, len(rows))
	for i := range rows {
		out[i] = *s.enrich(ctx, &rows[i], userID, isAdmin)
	}
	return out, nil
}

// Update 更新记录。
func (s *Service) Update(ctx context.Context, id uuid.UUID, name, path, gitURL, branch, visibility *string, groupID *uuid.UUID) (*Project, error) {
	m, err := s.stores.Project.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		m.Name = *name
	}
	if path != nil {
		code := NormalizeProjectCode(*path)
		if code == "" {
			return nil, fmt.Errorf("项目编码不能为空")
		}
		if err := ValidateProjectCode(code); err != nil {
			return nil, err
		}
		exists, err := s.stores.Project.ExistsByPath(ctx, code, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, fmt.Errorf("项目编码 %q 已被使用", code)
		}
		m.Path = code
	}
	if gitURL != nil {
		m.GitURL = *gitURL
	}
	if branch != nil {
		m.GitBranch = *branch
	}
	if visibility != nil && *visibility != "" {
		m.Visibility = *visibility
	}
	if groupID != nil {
		m.GroupID = groupID
	}
	if err := s.stores.Project.Save(ctx, m); err != nil {
		return nil, err
	}
	return toDTO(m), nil
}

// Delete 删除记录。
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.stores.Project.Delete(ctx, id)
}

// ProjectWorkspaceKey 返回项目工作区目录键（项目编码）；无编码时返回错误，不使用 UUID 兜底。
func (s *Service) ProjectWorkspaceKey(ctx context.Context, projectID uuid.UUID) (string, error) {
	m, err := s.stores.Project.GetByID(ctx, projectID)
	if err != nil {
		return "", err
	}
	code := NormalizeProjectCode(m.Path)
	if code == "" {
		return "", fmt.Errorf("项目未配置编码")
	}
	if err := ValidateProjectCode(code); err != nil {
		return "", err
	}
	return code, nil
}

// enrich 为实体补充当前用户权限等扩展字段。
func (s *Service) enrich(ctx context.Context, m *models.Project, userID uuid.UUID, isAdmin bool) *Project {
	p := toDTO(m)
	enforcer := iam.NewEnforcer(s.stores.IAM)
	role, isMember, _ := enforcer.EffectiveRole(ctx, userID, m.ID)
	if isAdmin {
		p.CurrentUserRole = new(iam.RoleOwner)
		p.Permissions = new(iam.PermissionsForRole(iam.RoleOwner))
		return p
	}
	if isMember {
		p.CurrentUserRole = &role
		p.Permissions = new(iam.PermissionsForRole(role))
		return p
	}
	if m.Visibility == VisibilityInternal || m.Visibility == VisibilityPublic {
		p.Permissions = new(iam.GuestPermissions())
	}
	return p
}

// toDTO 将数据库模型转换为 API DTO。
func toDTO(m *models.Project) *Project {
	vis := m.Visibility
	if vis == "" {
		vis = VisibilityPrivate
	}
	return &Project{
		ID: m.ID, Name: m.Name, Path: m.Path, GitURL: m.GitURL, GitBranch: m.GitBranch,
		Visibility: vis, GroupID: m.GroupID, OwnerID: m.OwnerID, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}
