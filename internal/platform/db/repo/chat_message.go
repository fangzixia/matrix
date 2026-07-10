package repo

import (
	"context"
	"time"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChatMessageRepo 封装 ChatMessage 表持久化操作。
type ChatMessageRepo struct {
	db *gorm.DB
}

// NewChatMessageRepo 创建 ChatMessageRepo。
func NewChatMessageRepo(db *gorm.DB) *ChatMessageRepo {
	return &ChatMessageRepo{db: db}
}

// ListBySession 按会话 ID 列出消息。
func (r *ChatMessageRepo) ListBySession(ctx context.Context, sessionID uuid.UUID) ([]models.ChatMessage, error) {
	var rows []models.ChatMessage
	if err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("created_at asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetBySessionAndID 按会话与消息 ID 查询。
func (r *ChatMessageRepo) GetBySessionAndID(ctx context.Context, sessionID, messageID uuid.UUID) (*models.ChatMessage, error) {
	var row models.ChatMessage
	if err := r.db.WithContext(ctx).First(&row, "id = ? AND session_id = ?", messageID, sessionID).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// CountByRunID 统计 Run 关联的助手消息数。
func (r *ChatMessageRepo) CountByRunID(ctx context.Context, runID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.ChatMessage{}).Where("run_id = ?", runID).Count(&count).Error
	return count, err
}

// Create 创建消息。
func (r *ChatMessageRepo) Create(ctx context.Context, row *models.ChatMessage) error {
	return r.db.WithContext(ctx).Create(row).Error
}

// CreateUserWithActiveLeaf 插入用户消息并更新会话 active_leaf_id。
func (r *ChatMessageRepo) CreateUserWithActiveLeaf(ctx context.Context, row *models.ChatMessage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		return tx.Model(&models.ChatSession{}).Where("id = ?", row.SessionID).Updates(map[string]any{
			"active_leaf_id": row.ID,
			"updated_at":     time.Now(),
		}).Error
	})
}

// CreateAssistantWithActiveLeaf 插入助手消息并更新会话 active_leaf_id。
func (r *ChatMessageRepo) CreateAssistantWithActiveLeaf(ctx context.Context, row *models.ChatMessage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		return tx.Model(&models.ChatSession{}).Where("id = ?", row.SessionID).Updates(map[string]any{
			"active_leaf_id": row.ID,
			"updated_at":     time.Now(),
		}).Error
	})
}
