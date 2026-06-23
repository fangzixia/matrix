package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Group 是用户组，用于项目权限继承。
type Group struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"size:255;not null"`
	OwnerID   uuid.UUID `gorm:"type:uuid;index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BeforeCreate 在 Group 入库前生成 UUID 主键。
func (g *Group) BeforeCreate(tx *gorm.DB) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	return nil
}

// GroupMember 是用户组成员关系。
type GroupMember struct {
	GroupID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Role      string    `gorm:"size:32;not null"`
	CreatedAt time.Time
}
