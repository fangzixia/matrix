package repo

import (
	"context"
	"time"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
)

// NotificationStore 封装通知持久化。
type NotificationStore struct {
	c *catalog
}

func newNotificationStore(c *catalog) *NotificationStore { return &NotificationStore{c: c} }

func (s *NotificationStore) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]models.Notification, error) {
	return s.c.notification.ListByUser(ctx, userID, limit)
}

func (s *NotificationStore) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.c.notification.UnreadCount(ctx, userID)
}

func (s *NotificationStore) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	return s.c.notification.MarkRead(ctx, userID, id, time.Now())
}

func (s *NotificationStore) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.c.notification.MarkAllRead(ctx, userID, time.Now())
}

func (s *NotificationStore) Create(ctx context.Context, userID uuid.UUID, kind, title, body, link string) (*models.Notification, error) {
	m := models.Notification{UserID: userID, Kind: kind, Title: title, Body: body, Link: link}
	if err := s.c.notification.Create(ctx, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
