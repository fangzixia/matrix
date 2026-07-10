package repo

import (
	"context"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RunStepRepo 封装 RunStep 表持久化操作。
type RunStepRepo struct {
	db *gorm.DB
}

// NewRunStepRepo 创建 RunStepRepo。
func NewRunStepRepo(db *gorm.DB) *RunStepRepo {
	return &RunStepRepo{db: db}
}

// ListByRunID 按 Run ID 列出步骤。
func (r *RunStepRepo) ListByRunID(ctx context.Context, runID uuid.UUID) ([]models.RunStep, error) {
	var rows []models.RunStep
	if err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("sequence asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
