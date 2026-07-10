package repo

import (
	"context"
	"time"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PlanRepo 封装 Plan 表持久化操作。
type PlanRepo struct {
	db *gorm.DB
}

// NewPlanRepo 创建 PlanRepo。
func NewPlanRepo(db *gorm.DB) *PlanRepo {
	return &PlanRepo{db: db}
}

// GetByProjectAndPath 按项目与路径查询计划索引。
func (r *PlanRepo) GetByProjectAndPath(ctx context.Context, projectID uuid.UUID, path string) (*models.Plan, error) {
	var row models.Plan
	if err := r.db.WithContext(ctx).Where("project_id = ? AND path = ?", projectID, path).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// GetByProjectPathAndRepo 按项目、路径与可选仓库查询。
func (r *PlanRepo) GetByProjectPathAndRepo(ctx context.Context, projectID uuid.UUID, path string, repositoryID *uuid.UUID) (*models.Plan, error) {
	q := r.db.WithContext(ctx).Where("project_id = ? AND path = ?", projectID, path)
	if repositoryID != nil {
		q = q.Where("repository_id = ? OR repository_id IS NULL", *repositoryID)
	}
	var row models.Plan
	if err := q.First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// Save 保存计划索引。
func (r *PlanRepo) Save(ctx context.Context, row *models.Plan) error {
	return r.db.WithContext(ctx).Save(row).Error
}

// Create 创建计划索引。
func (r *PlanRepo) Create(ctx context.Context, row *models.Plan) error {
	return r.db.WithContext(ctx).Create(row).Error
}

// UpsertAfterRun 在 Run 成功后插入或更新计划索引。
func (r *PlanRepo) UpsertAfterRun(ctx context.Context, row *models.Plan) error {
	existing, err := r.GetByProjectPathAndRepo(ctx, row.ProjectID, row.Path, row.RepositoryID)
	if err == nil {
		existing.Title = row.Title
		existing.RunID = row.RunID
		existing.UpdatedAt = row.UpdatedAt
		if row.RepositoryID != nil {
			existing.RepositoryID = row.RepositoryID
		}
		return r.Save(ctx, existing)
	}
	if !IsNotFound(err) {
		return err
	}
	return r.Create(ctx, row)
}

// ApproveExisting 批准已有计划索引。
func (r *PlanRepo) ApproveExisting(ctx context.Context, row *models.Plan, now time.Time) error {
	row.Status = "approved"
	row.UpdatedAt = now
	return r.Save(ctx, row)
}
