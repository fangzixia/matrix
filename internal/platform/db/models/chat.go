package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChatSession 是项目内 Chat 会话与消息 JSON。
type ChatSession struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID uuid.UUID `gorm:"type:uuid;index;not null"`
	Title     string    `gorm:"size:512"`
	Messages  string    `gorm:"type:jsonb"`
	CreatedBy uuid.UUID `gorm:"type:uuid;index"`
	UpdatedAt time.Time
	CreatedAt time.Time
}

// BeforeCreate 在 ChatSession 入库前生成 UUID 主键。
func (c *ChatSession) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}
