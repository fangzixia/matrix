package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"matrix/internal/ai/query"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/logging"
)

// buildChatRunMessages 从 chat_messages 表重建 Chat Run 的 LLM 输入。
func (s *Service) buildChatRunMessages(ctx context.Context, m *models.Run) ([]query.Message, error) {
	if m.ChatSessionID == nil || m.ChatUserMessageID == nil {
		return nil, errors.New("chat run 缺少 session 关联")
	}
	var userRow models.ChatMessage
	if err := s.db.WithContext(ctx).First(&userRow, "id = ? AND session_id = ?",
		*m.ChatUserMessageID, *m.ChatSessionID).Error; err != nil {
		return nil, fmt.Errorf("未找到用户消息: %w", err)
	}
	parentID := ""
	if userRow.ParentID != nil {
		parentID = userRow.ParentID.String()
	}
	history, err := s.HistoryForParentDB(ctx, *m.ChatSessionID, parentID)
	if err != nil {
		return nil, err
	}
	var attachments []query.MessageAttachment
	if userRow.Attachments != "" && userRow.Attachments != "null" {
		_ = json.Unmarshal([]byte(userRow.Attachments), &attachments)
	}
	userMsg := query.Message{
		Role: query.RoleUser, Content: userRow.Content, Attachments: attachments,
	}
	out := append(history, userMsg)
	logging.Info("run: chat 消息已构建",
		"run_id", m.ID,
		"session_id", *m.ChatSessionID,
		"user_message_id", *m.ChatUserMessageID,
		"history_len", len(history),
		"total_messages", len(out),
		"attachments", len(attachments),
		"user_content_len", len(userRow.Content),
	)
	return out, nil
}
