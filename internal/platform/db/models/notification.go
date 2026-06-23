package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Notification 是用户站内通知。
type Notification struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null"`
	Kind      string    `gorm:"size:64;not null"`
	Title     string    `gorm:"size:512"`
	Body      string    `gorm:"type:text"`
	Link      string    `gorm:"size:1024"`
	ReadAt    *time.Time
	CreatedAt time.Time
}

// BeforeCreate 在 Notification 入库前生成 UUID 主键。
func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}
