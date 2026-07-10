package repo

import (
	"context"
	"time"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
)

// ChatStore 封装 Chat 会话与消息的持久化操作。
type ChatStore struct {
	c *catalog
}

func newChatStore(c *catalog) *ChatStore { return &ChatStore{c: c} }

// CreateSessionParams 创建会话参数。
type CreateSessionParams struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	Title     string
	ModelID   string
	CreatedBy uuid.UUID
}

// CreateSession 创建 Chat 会话。
func (s *ChatStore) CreateSession(ctx context.Context, p CreateSessionParams) (*models.ChatSession, error) {
	m := models.ChatSession{
		ID: p.ID, ProjectID: p.ProjectID, Title: p.Title,
		ModelID: p.ModelID, CreatedBy: p.CreatedBy,
	}
	if err := s.c.chatSession.Create(ctx, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// GetSessionByProject 按项目与会话 ID 查询。
func (s *ChatStore) GetSessionByProject(ctx context.Context, projectID, sessionID uuid.UUID) (*models.ChatSession, error) {
	return s.c.chatSession.GetByProject(ctx, projectID, sessionID)
}

// GetSession 按 ID 查询会话。
func (s *ChatStore) GetSession(ctx context.Context, sessionID uuid.UUID) (*models.ChatSession, error) {
	return s.c.chatSession.GetByID(ctx, sessionID)
}

// UpdateSession 更新会话元数据。
func (s *ChatStore) UpdateSession(ctx context.Context, sessionID uuid.UUID, title, modelID string) (*models.ChatSession, error) {
	updates := map[string]any{"updated_at": time.Now()}
	if title != "" {
		updates["title"] = title
	}
	if modelID != "" {
		updates["model_id"] = modelID
	}
	if err := s.c.chatSession.UpdateFields(ctx, sessionID, updates); err != nil {
		return nil, err
	}
	return s.c.chatSession.GetByID(ctx, sessionID)
}

// ListSessionsByProject 列出项目下全部 Chat 会话。
func (s *ChatStore) ListSessionsByProject(ctx context.Context, projectID uuid.UUID) ([]models.ChatSession, error) {
	return s.c.chatSession.ListByProject(ctx, projectID)
}

// DeleteSession 删除会话及其消息。
func (s *ChatStore) DeleteSession(ctx context.Context, projectID, sessionID uuid.UUID) error {
	if _, err := s.c.chatSession.GetByProject(ctx, projectID, sessionID); err != nil {
		return err
	}
	return s.c.chatSession.DeleteWithMessages(ctx, sessionID)
}

// ListMessages 列出会话消息。
func (s *ChatStore) ListMessages(ctx context.Context, sessionID uuid.UUID) ([]models.ChatMessage, error) {
	return s.c.chatMessage.ListBySession(ctx, sessionID)
}

// GetUserMessage 按会话与用户消息 ID 查询。
func (s *ChatStore) GetUserMessage(ctx context.Context, sessionID, messageID uuid.UUID) (*models.ChatMessage, error) {
	return s.c.chatMessage.GetBySessionAndID(ctx, sessionID, messageID)
}

// InsertUserMessageParams 插入用户消息参数。
type InsertUserMessageParams struct {
	SessionID   uuid.UUID
	MessageID   uuid.UUID
	ParentID    *uuid.UUID
	Content     string
	Attachments string
}

// InsertUserMessage 插入用户消息并更新 active_leaf_id。
func (s *ChatStore) InsertUserMessage(ctx context.Context, p InsertUserMessageParams) error {
	row := models.ChatMessage{
		ID: p.MessageID, SessionID: p.SessionID, ParentID: p.ParentID,
		Role: "user", Content: p.Content, Attachments: p.Attachments,
		CreatedAt: time.Now(),
	}
	return s.c.chatMessage.CreateUserWithActiveLeaf(ctx, &row)
}

// InsertAssistantMessageParams 插入助手消息参数。
type InsertAssistantMessageParams struct {
	SessionID     uuid.UUID
	AssistantID   uuid.UUID
	UserMessageID uuid.UUID
	RunID         uuid.UUID
	Content       string
}

// InsertAssistantMessage 插入助手消息并更新 active_leaf_id。
func (s *ChatStore) InsertAssistantMessage(ctx context.Context, p InsertAssistantMessageParams) (uuid.UUID, error) {
	row := models.ChatMessage{
		ID: p.AssistantID, SessionID: p.SessionID, ParentID: &p.UserMessageID,
		Role: "assistant", Content: p.Content, RunID: &p.RunID,
		CreatedAt: time.Now(),
	}
	if err := s.c.chatMessage.CreateAssistantWithActiveLeaf(ctx, &row); err != nil {
		return uuid.Nil, err
	}
	return p.AssistantID, nil
}
