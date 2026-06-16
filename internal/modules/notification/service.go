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

type Service struct {
	db  *gorm.DB
	hub *events.Hub
}

func NewService(db *gorm.DB, hub *events.Hub) *Service {
	return &Service{db: db, hub: hub}
}

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

func (s *Service) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).Count(&n).Error
	return n, err
}

func (s *Service) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("read_at", now).Error
}

func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", now).Error
}

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

func (s *Service) NotifyRunStatus(ctx context.Context, userID uuid.UUID, projectID, runID uuid.UUID, status, title string) {
	link := fmt.Sprintf("/projects/%s/runs/%s", projectID, runID)
	body := fmt.Sprintf("Run %s", status)
	_, _ = s.Create(ctx, userID, "run:"+status, title, body, link)
}

func toDTO(m *models.Notification) DTO {
	return DTO{
		ID: m.ID, UserID: m.UserID, Kind: m.Kind, Title: m.Title, Body: m.Body,
		Link: m.Link, ReadAt: m.ReadAt, CreatedAt: m.CreatedAt,
	}
}
