package repo

import (
	"context"
	"time"

	"matrix/internal/platform/db/models"

	"gorm.io/gorm"
)

// SessionRepo 封装 Session 表持久化操作。
type SessionRepo struct {
	db *gorm.DB
}

// NewSessionRepo 创建 SessionRepo。
func NewSessionRepo(db *gorm.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

// Create 创建 Session。
func (r *SessionRepo) Create(ctx context.Context, m *models.Session) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// GetValidByTokenHash 按 token 哈希查询未过期 Session。
func (r *SessionRepo) GetValidByTokenHash(ctx context.Context, hash string, now time.Time) (*models.Session, error) {
	var sess models.Session
	if err := r.db.WithContext(ctx).Where("token_hash = ? AND expires_at > ?", hash, now).First(&sess).Error; err != nil {
		return nil, err
	}
	return &sess, nil
}

// DeleteByTokenHash 按 token 哈希吊销 Session。
func (r *SessionRepo) DeleteByTokenHash(ctx context.Context, hash string) error {
	return r.db.WithContext(ctx).Where("token_hash = ?", hash).Delete(&models.Session{}).Error
}
