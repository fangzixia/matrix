package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Project 是 Git 绑定的业务项目。
type Project struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Name       string     `gorm:"size:255;not null"`
	Path       string     `gorm:"size:255;index"`
	GitURL     string     `gorm:"size:512"`
	GitBranch  string     `gorm:"size:128;default:main"`
	Visibility string     `gorm:"size:16;default:private;index"`
	GroupID    *uuid.UUID `gorm:"type:uuid;index"`
	OwnerID    uuid.UUID  `gorm:"type:uuid;index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// BeforeCreate 在 Project 入库前生成 UUID 主键。
func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// ProjectMember 是项目成员与角色。
type ProjectMember struct {
	ProjectID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Role      string    `gorm:"size:32;not null"`
	CreatedAt time.Time
}

// ProjectRepository 是项目下的 Git 仓库绑定。
type ProjectRepository struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID     uuid.UUID `gorm:"type:uuid;index;not null"`
	Name          string    `gorm:"size:128;not null"`
	GitURL        string    `gorm:"size:512"`
	GitBranch     string    `gorm:"size:128;default:main"`
	IsDefault     bool      `gorm:"default:false"`
	AuthType      string    `gorm:"size:32"`
	CredentialRef string    `gorm:"size:512"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// BeforeCreate 在 ProjectRepository 入库前生成 UUID 主键。
func (r *ProjectRepository) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}
