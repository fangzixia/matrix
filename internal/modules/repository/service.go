package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"matrix/internal/platform/db/models"
)

type DTO struct {
	ID            uuid.UUID `json:"id"`
	ProjectID     uuid.UUID `json:"project_id"`
	Name          string    `json:"name"`
	GitURL        string    `json:"git_url"`
	GitBranch     string    `json:"git_branch"`
	IsDefault     bool      `json:"is_default"`
	AuthType      string    `json:"auth_type,omitempty"`
	CredentialRef string    `json:"credential_ref,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateInput struct {
	Name      string `json:"name"`
	GitURL    string `json:"git_url"`
	GitBranch string `json:"git_branch"`
	IsDefault bool   `json:"is_default"`
}

type UpdateInput struct {
	Name      *string `json:"name"`
	GitURL    *string `json:"git_url"`
	GitBranch *string `json:"git_branch"`
	IsDefault *bool   `json:"is_default"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]DTO, error) {
	var rows []models.ProjectRepository
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("is_default desc, created_at asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]DTO, len(rows))
	for i := range rows {
		out[i] = toDTO(&rows[i])
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*DTO, error) {
	var m models.ProjectRepository
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return new(toDTO(&m)), nil
}

func (s *Service) GetDefault(ctx context.Context, projectID uuid.UUID) (*DTO, error) {
	var m models.ProjectRepository
	err := s.db.WithContext(ctx).Where("project_id = ? AND is_default = ?", projectID, true).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		err = s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at asc").First(&m).Error
	}
	if err != nil {
		return nil, err
	}
	return new(toDTO(&m)), nil
}

func (s *Service) Create(ctx context.Context, projectID uuid.UUID, in CreateInput) (*DTO, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errors.New("仓库名称不能为空")
	}
	branch := in.GitBranch
	if branch == "" {
		branch = "main"
	}
	m := models.ProjectRepository{
		ProjectID: projectID, Name: name, GitURL: in.GitURL, GitBranch: branch, IsDefault: in.IsDefault,
	}
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return nil, err
	}
	if in.IsDefault {
		_ = s.clearOtherDefaults(ctx, projectID, m.ID)
	}
	return new(toDTO(&m)), nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*DTO, error) {
	var m models.ProjectRepository
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if in.Name != nil {
		m.Name = strings.TrimSpace(*in.Name)
	}
	if in.GitURL != nil {
		m.GitURL = *in.GitURL
	}
	if in.GitBranch != nil {
		m.GitBranch = *in.GitBranch
	}
	if in.IsDefault != nil && *in.IsDefault {
		m.IsDefault = true
		_ = s.clearOtherDefaults(ctx, m.ProjectID, m.ID)
	}
	if err := s.db.WithContext(ctx).Save(&m).Error; err != nil {
		return nil, err
	}
	return new(toDTO(&m)), nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	var m models.ProjectRepository
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return err
	}
	if m.IsDefault {
		var count int64
		s.db.WithContext(ctx).Model(&models.ProjectRepository{}).Where("project_id = ?", m.ProjectID).Count(&count)
		if count <= 1 {
			return errors.New("不能删除唯一的默认仓库")
		}
	}
	return s.db.WithContext(ctx).Delete(&m).Error
}

func (s *Service) SeedDefault(ctx context.Context, projectID uuid.UUID, gitURL, branch string) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.ProjectRepository{}).Where("project_id = ?", projectID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if branch == "" {
		branch = "main"
	}
	m := models.ProjectRepository{
		ProjectID: projectID, Name: "default", GitURL: gitURL, GitBranch: branch, IsDefault: true,
	}
	return s.db.WithContext(ctx).Create(&m).Error
}

func (s *Service) MigrateLegacyProjects(ctx context.Context) error {
	var projects []models.Project
	if err := s.db.WithContext(ctx).Find(&projects).Error; err != nil {
		return err
	}
	for _, p := range projects {
		if err := s.SeedDefault(ctx, p.ID, p.GitURL, p.GitBranch); err != nil {
			return fmt.Errorf("project %s: %w", p.ID, err)
		}
	}
	return nil
}

func (s *Service) clearOtherDefaults(ctx context.Context, projectID, keepID uuid.UUID) error {
	return s.db.WithContext(ctx).Model(&models.ProjectRepository{}).
		Where("project_id = ? AND id <> ?", projectID, keepID).
		Update("is_default", false).Error
}

func toDTO(m *models.ProjectRepository) DTO {
	return DTO{
		ID: m.ID, ProjectID: m.ProjectID, Name: m.Name, GitURL: m.GitURL, GitBranch: m.GitBranch,
		IsDefault: m.IsDefault, AuthType: m.AuthType, CredentialRef: m.CredentialRef,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}
