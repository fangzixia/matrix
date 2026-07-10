package run

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"matrix/internal/ai/query"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/db/repo"

	"github.com/google/uuid"
)

// LoadSessionTree 从 chat_messages 表加载会话消息树。
func (s *Service) LoadSessionTree(ctx context.Context, session *models.ChatSession) (SessionMessages, error) {
	rows, err := s.stores.Chat.ListMessages(ctx, session.ID)
	if err != nil {
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
		node.ParentID = new(row.ParentID.String())
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
	session, err := s.stores.Chat.GetSession(ctx, sessionID)
	if err != nil {
		return SessionMessages{}, err
	}
	return s.LoadSessionTree(ctx, session)
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
	return s.stores.Chat.InsertUserMessage(ctx, repo.InsertUserMessageParams{
		SessionID: sessionID, MessageID: messageID, ParentID: parentUUID,
		Content: content, Attachments: encodeAttachments(attachments),
	})
}

// InsertChatAssistantMessage 在 Run 完成后插入 assistant 消息。
func (s *Service) InsertChatAssistantMessage(ctx context.Context, sessionID, userMessageID, runID uuid.UUID, content string) (uuid.UUID, error) {
	if strings.TrimSpace(content) == "" {
		content = "（无回复）"
	}
	return s.stores.Chat.InsertAssistantMessage(ctx, repo.InsertAssistantMessageParams{
		SessionID: sessionID, AssistantID: uuid.New(), UserMessageID: userMessageID,
		RunID: runID, Content: content,
	})
}
