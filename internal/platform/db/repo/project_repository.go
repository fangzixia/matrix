package repo

import (
	"context"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProjectRepositoryRepo 封装 ProjectRepository（Git 绑定）表持久化操作。
type ProjectRepositoryRepo struct {
	db *gorm.DB
}

// NewProjectRepositoryRepo 创建 ProjectRepositoryRepo。
func NewProjectRepositoryRepo(db *gorm.DB) *ProjectRepositoryRepo {
	return &ProjectRepositoryRepo{db: db}
}

// ListByProject 列出项目 Git 仓库绑定。
func (r *ProjectRepositoryRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.ProjectRepository, error) {
	var rows []models.ProjectRepository
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).
		Order("is_default desc, created_at asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetByProject 按项目与 ID 查询。
func (r *ProjectRepositoryRepo) GetByProject(ctx context.Context, projectID, id uuid.UUID) (*models.ProjectRepository, error) {
	var m models.ProjectRepository
	if err := r.db.WithContext(ctx).Where("id = ? AND project_id = ?", id, projectID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetDefault 返回项目默认仓库。
func (r *ProjectRepositoryRepo) GetDefault(ctx context.Context, projectID uuid.UUID) (*models.ProjectRepository, error) {
	var m models.ProjectRepository
	err := r.db.WithContext(ctx).Where("project_id = ? AND is_default = ?", projectID, true).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		err = r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at asc").First(&m).Error
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 创建 Git 仓库绑定。
func (r *ProjectRepositoryRepo) Create(ctx context.Context, m *models.ProjectRepository) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// CreateWithDefault 创建绑定并在需要时清除其他 default。
func (r *ProjectRepositoryRepo) CreateWithDefault(ctx context.Context, m *models.ProjectRepository, setDefault bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		if setDefault {
			return tx.Model(&models.ProjectRepository{}).
				Where("project_id = ? AND id <> ?", m.ProjectID, m.ID).
				Update("is_default", false).Error
		}
		return nil
	})
}

// Delete 删除 Git 仓库绑定。
func (r *ProjectRepositoryRepo) Delete(ctx context.Context, m *models.ProjectRepository) error {
	return r.db.WithContext(ctx).Delete(m).Error
}

// CountByProject 统计项目仓库数量。
func (r *ProjectRepositoryRepo) CountByProject(ctx context.Context, projectID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.ProjectRepository{}).Where("project_id = ?", projectID).Count(&count).Error
	return count, err
}
