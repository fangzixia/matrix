package repo

import (
	"context"
	"errors"
	"time"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChatSessionRepo 封装 ChatSession 表持久化操作。
type ChatSessionRepo struct {
	db *gorm.DB
}

// NewChatSessionRepo 创建 ChatSessionRepo。
func NewChatSessionRepo(db *gorm.DB) *ChatSessionRepo {
	return &ChatSessionRepo{db: db}
}

// Create 创建 Chat 会话。
func (r *ChatSessionRepo) Create(ctx context.Context, m *models.ChatSession) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// GetByProject 按项目与会话 ID 查询。
func (r *ChatSessionRepo) GetByProject(ctx context.Context, projectID, sessionID uuid.UUID) (*models.ChatSession, error) {
	var row models.ChatSession
	if err := r.db.WithContext(ctx).Where("id = ? AND project_id = ?", sessionID, projectID).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// GetByID 按 ID 查询会话。
func (r *ChatSessionRepo) GetByID(ctx context.Context, sessionID uuid.UUID) (*models.ChatSession, error) {
	var row models.ChatSession
	if err := r.db.WithContext(ctx).First(&row, "id = ?", sessionID).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// ListByProject 列出项目下全部 Chat 会话。
func (r *ChatSessionRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.ChatSession, error) {
	var rows []models.ChatSession
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("updated_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// UpdateFields 更新会话字段。
func (r *ChatSessionRepo) UpdateFields(ctx context.Context, sessionID uuid.UUID, updates map[string]any) error {
	return r.db.WithContext(ctx).Model(&models.ChatSession{}).Where("id = ?", sessionID).Updates(updates).Error
}

// SetActiveLeaf 更新 active_leaf_id 与 updated_at。
func (r *ChatSessionRepo) SetActiveLeaf(ctx context.Context, sessionID, leafID uuid.UUID) error {
	return r.UpdateFields(ctx, sessionID, map[string]any{
		"active_leaf_id": leafID,
		"updated_at":     time.Now(),
	})
}

// DeleteWithMessages 删除会话及其全部消息。
func (r *ChatSessionRepo) DeleteWithMessages(ctx context.Context, sessionID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).Delete(&models.ChatMessage{}).Error; err != nil {
			return err
		}
		res := tx.Delete(&models.ChatSession{}, "id = ?", sessionID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("会话不存在")
		}
		return nil
	})
}
