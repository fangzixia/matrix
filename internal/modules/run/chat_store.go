package run

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"matrix/internal/ai/query"
	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LoadSessionTree 从 chat_messages 表加载会话消息树。
func (s *Service) LoadSessionTree(ctx context.Context, session *models.ChatSession) (SessionMessages, error) {
	var rows []models.ChatMessage
	if err := s.db.WithContext(ctx).Where("session_id = ?", session.ID).Order("created_at asc").Find(&rows).Error; err != nil {
		return SessionMessages{}, err
	}
	nodes := make([]ChatMessageNode, 0, len(rows))
	for _, row := range rows {
		nodes = append(nodes, modelToChatNode(row))
	}
	activeLeaf := ""
	if session.ActiveLeafID != nil {
		activeLeaf = session.ActiveLeafID.String()
	}
	return SessionMessages{Version: chatMessagesVersion, ActiveLeafID: activeLeaf, Nodes: nodes}, nil
}

func modelToChatNode(row models.ChatMessage) ChatMessageNode {
	node := ChatMessageNode{
		ID:      row.ID.String(),
		Role:    row.Role,
		Content: row.Content,
	}
	if row.ParentID != nil {
		pid := row.ParentID.String()
		node.ParentID = &pid
	}
	if row.RunID != nil {
		node.RunID = row.RunID.String()
	}
	if row.Attachments != "" && row.Attachments != "null" {
		var atts []query.MessageAttachment
		_ = json.Unmarshal([]byte(row.Attachments), &atts)
		node.Attachments = atts
	}
	if !row.CreatedAt.IsZero() {
		node.CreatedAt = row.CreatedAt.UTC().Format(time.RFC3339)
	}
	return node
}

func encodeAttachments(atts []query.MessageAttachment) string {
	if len(atts) == 0 {
		return "[]"
	}
	b, err := json.Marshal(atts)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// HistoryForParentDB 从数据库按 parent_id 回溯祖先链并转为 query.Message。
func (s *Service) HistoryForParentDB(ctx context.Context, sessionID uuid.UUID, parentID string) ([]query.Message, error) {
	sm, err := s.loadSessionTreeByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return HistoryForParent(sm, parentID)
}

func (s *Service) loadSessionTreeByID(ctx context.Context, sessionID uuid.UUID) (SessionMessages, error) {
	var session models.ChatSession
	if err := s.db.WithContext(ctx).First(&session, "id = ?", sessionID).Error; err != nil {
		return SessionMessages{}, err
	}
	return s.LoadSessionTree(ctx, &session)
}

// ValidateParentDB 校验 parent_id 是否属于会话。
func (s *Service) ValidateParentDB(ctx context.Context, sessionID uuid.UUID, parentID string) error {
	sm, err := s.loadSessionTreeByID(ctx, sessionID)
	if err != nil {
		return err
	}
	return ValidateParent(sm, parentID)
}

// InsertChatUserMessage 插入用户消息并更新 active_leaf_id。
func (s *Service) InsertChatUserMessage(ctx context.Context, sessionID uuid.UUID, parentID string, content string, attachments []query.MessageAttachment, messageID uuid.UUID) error {
	var parentUUID *uuid.UUID
	parentID = strings.TrimSpace(parentID)
	if parentID != "" {
		pid, err := uuid.Parse(parentID)
		if err != nil {
			return fmt.Errorf("无效的 parent_id")
		}
		parentUUID = &pid
	}
	row := models.ChatMessage{
		ID:          messageID,
		SessionID:   sessionID,
		ParentID:    parentUUID,
		Role:        "user",
		Content:     content,
		Attachments: encodeAttachments(attachments),
		CreatedAt:   time.Now(),
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return tx.Model(&models.ChatSession{}).Where("id = ?", sessionID).Updates(map[string]any{
			"active_leaf_id": messageID,
			"updated_at":     time.Now(),
		}).Error
	})
}

// InsertChatAssistantMessage 在 Run 完成后插入 assistant 消息。
func (s *Service) InsertChatAssistantMessage(ctx context.Context, sessionID, userMessageID, runID uuid.UUID, content string) (uuid.UUID, error) {
	if strings.TrimSpace(content) == "" {
		content = "（无回复）"
	}
	assistantID := uuid.New()
	row := models.ChatMessage{
		ID:        assistantID,
		SessionID: sessionID,
		ParentID:  &userMessageID,
		Role:      "assistant",
		Content:   content,
		RunID:     &runID,
		CreatedAt: time.Now(),
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return tx.Model(&models.ChatSession{}).Where("id = ?", sessionID).Updates(map[string]any{
			"active_leaf_id": assistantID,
			"updated_at":     time.Now(),
		}).Error
	})
	return assistantID, err
}
