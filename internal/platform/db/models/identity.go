package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User 是平台用户账户。
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Username     string    `gorm:"uniqueIndex;size:64;not null"`
	Email        string    `gorm:"uniqueIndex;size:255;not null"`
	PasswordHash string    `gorm:"not null"`
	Name         string    `gorm:"size:128"`
	AvatarURL    string    `gorm:"size:512"`
	IsAdmin      bool      `gorm:"default:false"`
	State        string    `gorm:"size:16;default:active"`
	LastSignInAt *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// BeforeCreate 在 User 入库前生成 UUID 主键。
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// Session 是用户登录 Session 记录。
type Session struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	IP        string    `gorm:"size:64"`
	UserAgent string    `gorm:"size:512"`
	ExpiresAt time.Time `gorm:"index"`
	CreatedAt time.Time
}

// BeforeCreate 在 Session 入库前生成 UUID 主键。
func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
