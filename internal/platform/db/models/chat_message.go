package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChatMessage 是 Chat 会话中的一条消息（parent_id 树节点）。
type ChatMessage struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	SessionID   uuid.UUID  `gorm:"type:uuid;index:idx_chat_msg_session;not null"`
	ParentID    *uuid.UUID `gorm:"type:uuid;index"`
	Role        string     `gorm:"size:16;not null"`
	Content     string     `gorm:"type:text"`
	Attachments string     `gorm:"type:jsonb"`
	RunID       *uuid.UUID `gorm:"type:uuid;index"`
	CreatedAt   time.Time
}

// BeforeCreate 在 ChatMessage 入库前生成 UUID 主键。
func (m *ChatMessage) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.Attachments == "" {
		m.Attachments = "[]"
	}
	return nil
}
