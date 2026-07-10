package repo

import (
	"context"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProjectMemberRepo 封装 ProjectMember 表持久化操作。
type ProjectMemberRepo struct {
	db *gorm.DB
}

// NewProjectMemberRepo 创建 ProjectMemberRepo。
func NewProjectMemberRepo(db *gorm.DB) *ProjectMemberRepo {
	return &ProjectMemberRepo{db: db}
}

// Create 创建项目成员。
func (r *ProjectMemberRepo) Create(ctx context.Context, m *models.ProjectMember) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// Get 查询项目成员。
func (r *ProjectMemberRepo) Get(ctx context.Context, projectID, userID uuid.UUID) (*models.ProjectMember, error) {
	var m models.ProjectMember
	if err := r.db.WithContext(ctx).Where("project_id = ? AND user_id = ?", projectID, userID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ListByProject 列出项目成员。
func (r *ProjectMemberRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.ProjectMember, error) {
	var rows []models.ProjectMember
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Save 保存项目成员。
func (r *ProjectMemberRepo) Save(ctx context.Context, m *models.ProjectMember) error {
	return r.db.WithContext(ctx).Save(m).Error
}

// UpdateRole 更新成员角色。
func (r *ProjectMemberRepo) UpdateRole(ctx context.Context, projectID, userID uuid.UUID, role string) error {
	return r.db.WithContext(ctx).Model(&models.ProjectMember{}).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Update("role", role).Error
}

// Delete 删除项目成员。
func (r *ProjectMemberRepo) Delete(ctx context.Context, projectID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Delete(&models.ProjectMember{}).Error
}
