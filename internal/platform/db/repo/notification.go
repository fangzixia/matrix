package repo

import (
	"context"
	"time"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationRepo 封装 Notification 表持久化操作。
type NotificationRepo struct {
	db *gorm.DB
}

// NewNotificationRepo 创建 NotificationRepo。
func NewNotificationRepo(db *gorm.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}

// ListByUser 列出用户通知。
func (r *NotificationRepo) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]models.Notification, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []models.Notification
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("created_at desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// UnreadCount 返回未读通知数。
func (r *NotificationRepo) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).Count(&n).Error
	return n, err
}

// MarkRead 标记单条通知已读。
func (r *NotificationRepo) MarkRead(ctx context.Context, userID, id uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("read_at", now).Error
}

// MarkAllRead 标记全部通知已读。
func (r *NotificationRepo) MarkAllRead(ctx context.Context, userID uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", now).Error
}

// Create 创建通知。
func (r *NotificationRepo) Create(ctx context.Context, m *models.Notification) error {
	return r.db.WithContext(ctx).Create(m).Error
}
