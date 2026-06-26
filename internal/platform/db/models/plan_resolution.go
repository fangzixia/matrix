package models

import (
	"time"

	"github.com/google/uuid"
)

// PlanResolution 是计划确认项的用户决议（plan_id + item_key 唯一）。
type PlanResolution struct {
	PlanID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	ItemKey    string    `gorm:"size:128;primaryKey"`
	Resolution string    `gorm:"type:text;not null"`
	CreatedAt  time.Time
}
