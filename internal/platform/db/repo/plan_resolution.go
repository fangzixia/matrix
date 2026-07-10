package repo

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PlanResolutionRepo 封装 PlanResolution 表持久化操作。
type PlanResolutionRepo struct {
	db *gorm.DB
}

// NewPlanResolutionRepo 创建 PlanResolutionRepo。
func NewPlanResolutionRepo(db *gorm.DB) *PlanResolutionRepo {
	return &PlanResolutionRepo{db: db}
}

// ReplaceAll 全量替换计划的确认项。
func (r *PlanResolutionRepo) ReplaceAll(ctx context.Context, planID uuid.UUID, resolutions map[string]string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return replacePlanResolutions(ctx, tx, planID, resolutions)
	})
}
