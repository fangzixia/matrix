package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Plan 是项目计划文档元数据索引（正文存于项目 docs/plans/PLAN-*.md）。
type Plan struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ProjectID    uuid.UUID  `gorm:"type:uuid;index;not null"`
	RepositoryID *uuid.UUID `gorm:"type:uuid;index"`
	RunID        *uuid.UUID `gorm:"type:uuid;index"`
	Path         string     `gorm:"size:512;not null"`
	Title        string     `gorm:"size:512"`
	Status       string     `gorm:"size:32;default:draft;index"`
	Resolutions  string     `gorm:"type:text"`
	UpdatedAt    time.Time
	CreatedAt    time.Time
}

// TableName 指定 GORM 表名。
func (p *Plan) TableName() string { return "plans" }

// BeforeCreate 在 Plan 入库前生成 UUID 主键。
func (p *Plan) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// Artifact 是评测或构建产物元数据索引（正文存于 docs/evaluations/EVAL-*.md）。
type Artifact struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ProjectID    uuid.UUID  `gorm:"type:uuid;index;not null"`
	RepositoryID *uuid.UUID `gorm:"type:uuid;index"`
	RunID        *uuid.UUID `gorm:"type:uuid;index"`
	Kind         string     `gorm:"size:64;not null"`
	Path         string     `gorm:"size:512;not null"`
	PlanPath     string     `gorm:"size:512"`
	Title        string     `gorm:"size:512"`
	CreatedAt    time.Time
}

// BeforeCreate 在 Artifact 入库前生成 UUID 主键。
func (a *Artifact) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
