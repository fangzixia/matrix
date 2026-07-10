package repo

import (
	"context"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RunViewRepo 封装 RunView 表持久化操作。
type RunViewRepo struct {
	db *gorm.DB
}

// NewRunViewRepo 创建 RunViewRepo。
func NewRunViewRepo(db *gorm.DB) *RunViewRepo {
	return &RunViewRepo{db: db}
}

// Save 保存或更新 Run 视图快照。
func (r *RunViewRepo) Save(ctx context.Context, row *models.RunView) error {
	return r.db.WithContext(ctx).Save(row).Error
}

// GetByRunID 按 Run ID 读取视图快照。
func (r *RunViewRepo) GetByRunID(ctx context.Context, runID uuid.UUID) (*models.RunView, error) {
	var row models.RunView
	err := r.db.WithContext(ctx).First(&row, "run_id = ?", runID).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}
