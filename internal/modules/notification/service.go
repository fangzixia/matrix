// Package notification 站内通知持久化与 SSE 实时推送。
package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"matrix/internal/platform/db/models"
	"matrix/internal/platform/events"
)

// DTO 是通知 API 返回的数据传输对象。
type DTO struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	Kind      string     `json:"kind"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Link      string     `json:"link,omitempty"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Service 管理站内通知的持久化与 SSE 实时推送。
type Service struct {
	db  *gorm.DB
	hub *events.Hub
}

// NewService 创建通知服务实例。
func NewService(db *gorm.DB, hub *events.Hub) *Service {
	return &Service{db: db, hub: hub}
}

// List 返回列表。
func (s *Service) List(ctx context.Context, userID uuid.UUID, limit int) ([]DTO, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []models.Notification
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("created_at desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]DTO, len(rows))
	for i := range rows {
		out[i] = toDTO(&rows[i])
	}
	return out, nil
}

// UnreadCount 返回未读通知数量。
func (s *Service) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).Count(&n).Error
	return n, err
}

// MarkRead 标记单条通知为已读。
func (s *Service) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("read_at", now).Error
}

// MarkAllRead 标记全部通知为已读。
func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", now).Error
}

// Create 创建记录。
func (s *Service) Create(ctx context.Context, userID uuid.UUID, kind, title, body, link string) (*DTO, error) {
	m := models.Notification{UserID: userID, Kind: kind, Title: title, Body: body, Link: link}
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return nil, err
	}
	d := toDTO(&m)
	if s.hub != nil {
		s.hub.PublishNotification(userID.String(), map[string]any{
			"type": "notification", "notification": d,
		})
	}
	return &d, nil
}

// NotifyRunStatus 在四阶段任务状态变更时推送通知。
func (s *Service) NotifyRunStatus(ctx context.Context, userID uuid.UUID, projectID, runID uuid.UUID, runKind, status, title string) {
	link := fmt.Sprintf("/projects/%s/%s/%s", projectID, runKind, runID)
	body := fmt.Sprintf("任务 %s", status)
	_, _ = s.Create(ctx, userID, "task:"+status, title, body, link)
}

func toDTO(m *models.Notification) DTO {
	return DTO{
		ID: m.ID, UserID: m.UserID, Kind: m.Kind, Title: m.Title, Body: m.Body,
		Link: m.Link, ReadAt: m.ReadAt, CreatedAt: m.CreatedAt,
	}
}
