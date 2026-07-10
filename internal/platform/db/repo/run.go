package repo

import (
	"context"
	"time"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RunRepo 封装 Run 表持久化操作。
type RunRepo struct {
	db *gorm.DB
}

// NewRunRepo 创建 RunRepo。
func NewRunRepo(db *gorm.DB) *RunRepo {
	return &RunRepo{db: db}
}

// ListByProject 按项目（及可选 kind）列出 Run。
func (r *RunRepo) ListByProject(ctx context.Context, projectID uuid.UUID, kind string) ([]models.Run, error) {
	q := r.db.WithContext(ctx).Where("project_id = ?", projectID)
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	var rows []models.Run
	if err := q.Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetByID 按 ID 查询 Run。
func (r *RunRepo) GetByID(ctx context.Context, runID uuid.UUID) (*models.Run, error) {
	var m models.Run
	if err := r.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByProject 按项目与 Run ID 查询。
func (r *RunRepo) GetByProject(ctx context.Context, projectID, runID uuid.UUID) (*models.Run, error) {
	var m models.Run
	if err := r.db.WithContext(ctx).Where("id = ? AND project_id = ?", runID, projectID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 创建 Run 记录。
func (r *RunRepo) Create(ctx context.Context, m *models.Run) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// UpdateFields 按 ID 更新指定字段。
func (r *RunRepo) UpdateFields(ctx context.Context, runID uuid.UUID, updates map[string]any) error {
	return r.db.WithContext(ctx).Model(&models.Run{}).Where("id = ?", runID).Updates(updates).Error
}

// UpdateModel 更新 Run 模型指定字段。
func (r *RunRepo) UpdateModel(ctx context.Context, m *models.Run, updates map[string]any) error {
	return r.db.WithContext(ctx).Model(m).Updates(updates).Error
}

// MarkRunning 将 Run 标记为运行中。
func (r *RunRepo) MarkRunning(ctx context.Context, runID uuid.UUID, now time.Time) error {
	return r.UpdateFields(ctx, runID, map[string]any{
		"status": "running", "started_at": now, "finished_at": nil, "error_message": "",
	})
}

// Finalize 写入 Run 终态。
func (r *RunRepo) Finalize(ctx context.Context, runID uuid.UUID, status, errMsg string, fin time.Time) error {
	return r.UpdateFields(ctx, runID, map[string]any{
		"status": status, "finished_at": fin, "error_message": errMsg,
	})
}

// Cancel 取消 Run。
func (r *RunRepo) Cancel(ctx context.Context, runID uuid.UUID, fin time.Time) error {
	return r.UpdateFields(ctx, runID, map[string]any{"status": "cancelled", "finished_at": fin})
}

// UpdateOutput 更新 Run 输出。
func (r *RunRepo) UpdateOutput(ctx context.Context, runID uuid.UUID, output string) error {
	return r.db.WithContext(ctx).Model(&models.Run{}).Where("id = ?", runID).Update("output", output).Error
}

// FindLatestSandboxForPlan 查找同 plan 下最近成功的沙箱来源 Run。
func (r *RunRepo) FindLatestSandboxForPlan(
	ctx context.Context,
	projectID uuid.UUID,
	repositoryID *uuid.UUID,
	planPath string,
	excludeRunID uuid.UUID,
	kinds []string,
) (*models.Run, error) {
	q := r.db.WithContext(ctx).Model(&models.Run{}).
		Where("project_id = ?", projectID).
		Where("file_path = ?", planPath).
		Where("kind IN ?", kinds).
		Where("status = ?", "succeeded").
		Where("sandbox_path <> ''").
		Where("id <> ?", excludeRunID)
	if repositoryID != nil {
		q = q.Where("repository_id = ?", *repositoryID)
	}
	var row models.Run
	if err := q.Order("finished_at DESC NULLS LAST, created_at DESC").First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}
