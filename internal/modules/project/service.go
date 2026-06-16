package project

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"matrix/internal/modules/iam"
	"matrix/internal/platform/db/models"
)

const (
	VisibilityPrivate  = "private"
	VisibilityInternal = "internal"
	VisibilityPublic   = "public"
)

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

type CreateInput struct {
	Name       string     `json:"name"`
	Path       string     `json:"path"`
	GitURL     string     `json:"git_url"`
	GitBranch  string     `json:"git_branch"`
	Visibility string     `json:"visibility"`
	GroupID    *uuid.UUID `json:"group_id"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, in CreateInput) (*Project, error) {
	branch := in.GitBranch
	if branch == "" {
		branch = "main"
	}
	vis := in.Visibility
	if vis == "" {
		vis = VisibilityPrivate
	}
	m := models.Project{
		Name: in.Name, Path: in.Path, GitURL: in.GitURL, GitBranch: branch,
		Visibility: vis, GroupID: in.GroupID, OwnerID: ownerID,
	}
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return nil, err
	}
	_ = s.db.WithContext(ctx).Create(&models.ProjectMember{
		ProjectID: m.ID, UserID: ownerID, Role: string(iam.RoleOwner),
	}).Error
	return s.enrich(ctx, &m, ownerID, false), nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Project, error) {
	var m models.Project
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return toDTO(&m), nil
}

func (s *Service) GetForUser(ctx context.Context, id, userID uuid.UUID, isAdmin bool) (*Project, error) {
	var m models.Project
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return s.enrich(ctx, &m, userID, isAdmin), nil
}

func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool, scope string) ([]Project, error) {
	var rows []models.Project
	q := s.db.WithContext(ctx).Model(&models.Project{})

	switch scope {
	case "explore":
		if !isAdmin {
			q = q.Where("visibility IN ?", []string{VisibilityInternal, VisibilityPublic})
		}
	case "starred":
		return []Project{}, nil
	default:
		if strings.HasPrefix(scope, "group/") {
			gid, err := uuid.Parse(strings.TrimPrefix(scope, "group/"))
			if err != nil {
				return nil, err
			}
			q = q.Where("group_id = ?", gid)
			if !isAdmin {
				q = q.Where(
					"owner_id = ? OR id IN (?) OR group_id IN (?)",
					userID,
					s.db.Model(&models.ProjectMember{}).Select("project_id").Where("user_id = ?", userID),
					s.db.Model(&models.GroupMember{}).Select("group_id").Where("user_id = ?", userID),
				)
			}
			break
		}
		if !isAdmin {
			q = q.Where(
				"owner_id = ? OR id IN (?) OR visibility IN ?",
				userID,
				s.db.Model(&models.ProjectMember{}).Select("project_id").Where("user_id = ?", userID),
				[]string{VisibilityInternal, VisibilityPublic},
			)
		}
	}

	if err := q.Order("updated_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Project, len(rows))
	for i := range rows {
		out[i] = *s.enrich(ctx, &rows[i], userID, isAdmin)
	}
	return out, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, name, path, gitURL, branch, visibility *string, groupID *uuid.UUID) (*Project, error) {
	var m models.Project
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if name != nil {
		m.Name = *name
	}
	if path != nil {
		m.Path = *path
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
	if err := s.db.WithContext(ctx).Save(&m).Error; err != nil {
		return nil, err
	}
	return toDTO(&m), nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&models.Project{}, "id = ?", id).Error
}

func (s *Service) enrich(ctx context.Context, m *models.Project, userID uuid.UUID, isAdmin bool) *Project {
	p := toDTO(m)
	enforcer := iam.NewEnforcer(s.db)
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
