package repo

import (
	"context"
	"strings"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func replacePlanResolutions(ctx context.Context, db *gorm.DB, planID uuid.UUID, resolutions map[string]string) error {
	if err := db.WithContext(ctx).Where("plan_id = ?", planID).Delete(&models.PlanResolution{}).Error; err != nil {
		return err
	}
	for key, val := range resolutions {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		row := models.PlanResolution{
			PlanID: planID, ItemKey: key, Resolution: strings.TrimSpace(val),
		}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// ApproveWithResolutions 原子写入计划批准状态并全量替换确认项。
func (r *PlanRepo) ApproveWithResolutions(ctx context.Context, row *models.Plan, createNew bool, resolutions map[string]string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if createNew {
			if err := tx.WithContext(ctx).Create(row).Error; err != nil {
				return err
			}
		} else if err := tx.WithContext(ctx).Save(row).Error; err != nil {
			return err
		}
		return replacePlanResolutions(ctx, tx, row.ID, resolutions)
	})
}
