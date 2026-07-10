// Package repository 项目 Git 仓库绑定与默认仓库种子数据。
package repository

import (
	"context"
	"errors"
	"fmt"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/db/repo"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DTO 是 Git 仓库 API 返回的数据传输对象。
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

// CreateInput 是创建 Git 仓库绑定时的请求参数。
type CreateInput struct {
	Name      string `json:"name"`
	GitURL    string `json:"git_url"`
	GitBranch string `json:"git_branch"`
	IsDefault bool   `json:"is_default"`
}

// Service 管理项目 Git 仓库绑定与默认仓库种子数据。
type Service struct {
	stores *repo.Stores
}

// NewService 创建仓库服务实例。
func NewService(stores *repo.Stores) *Service {
	return &Service{stores: stores}
}

// List 返回列表。
func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]DTO, error) {
	rows, err := s.stores.Git.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]DTO, len(rows))
	for i := range rows {
		out[i] = toDTO(&rows[i])
	}
	return out, nil
}

// GetForProject 返回项目内指定仓库。
func (s *Service) GetForProject(ctx context.Context, projectID, id uuid.UUID) (*DTO, error) {
	m, err := s.stores.Git.GetByProject(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	return new(toDTO(m)), nil
}

// GetDefault 返回项目默认仓库。
func (s *Service) GetDefault(ctx context.Context, projectID uuid.UUID) (*DTO, error) {
	m, err := s.stores.Git.GetDefault(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return new(toDTO(m)), nil
}

// Create 创建记录。
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
	if err := s.stores.Git.CreateWithDefault(ctx, &m, in.IsDefault); err != nil {
		return nil, err
	}
	return new(toDTO(&m)), nil
}

// DeleteForProject 删除项目内指定仓库。
func (s *Service) DeleteForProject(ctx context.Context, projectID, id uuid.UUID) error {
	m, err := s.stores.Git.GetByProject(ctx, projectID, id)
	if err != nil {
		return err
	}
	return s.deleteRow(ctx, m)
}

func (s *Service) deleteRow(ctx context.Context, m *models.ProjectRepository) error {
	if m.IsDefault {
		count, err := s.stores.Git.CountByProject(ctx, m.ProjectID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return errors.New("不能删除唯一的默认仓库")
		}
	}
	return s.stores.Git.Delete(ctx, m)
}

// SeedDefault 为项目创建默认仓库绑定。
func (s *Service) SeedDefault(ctx context.Context, projectID uuid.UUID, gitURL, branch string) error {
	count, err := s.stores.Git.CountByProject(ctx, projectID)
	if err != nil {
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
	return s.stores.Git.Create(ctx, &m)
}

// MigrateLegacyProjects 迁移旧版单仓库项目数据。
func (s *Service) MigrateLegacyProjects(ctx context.Context) error {
	projects, err := s.stores.Git.ListProjects(ctx)
	if err != nil {
		return err
	}
	for _, p := range projects {
		if err := s.SeedDefault(ctx, p.ID, p.GitURL, p.GitBranch); err != nil {
			return fmt.Errorf("project %s: %w", p.ID, err)
		}
	}
	return nil
}

// toDTO 将数据库模型转换为 API DTO。
func toDTO(m *models.ProjectRepository) DTO {
	return DTO{
		ID: m.ID, ProjectID: m.ProjectID, Name: m.Name, GitURL: m.GitURL, GitBranch: m.GitBranch,
		IsDefault: m.IsDefault, AuthType: m.AuthType, CredentialRef: m.CredentialRef,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}
