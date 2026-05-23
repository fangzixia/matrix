// Wails 暴露的自由对话入口（RunChatSession / ClearChatSession）。
package desktop

import (
	"fmt"
	"strings"

	"matrix/internal/query"
)

// ClearChatSession 删除某聊天的 Agent 上下文（内存 + 磁盘）。
func (b *Bridge) ClearChatSession(chatSessionID string) error {
	if chatSessionID == "" {
		return nil
	}
	return b.chatTranscripts.Clear(chatSessionID)
}

// GetChatSessions 返回当前工作区的对话历史列表。
func (b *Bridge) GetChatSessions() ([]ChatSession, error) {
	if b.chatSessionStore == nil {
		return nil, nil
	}
	return b.chatSessionStore.Load()
}

// SaveChatSessions 保存对话历史列表。
func (b *Bridge) SaveChatSessions(sessions []ChatSession) error {
	if b.chatSessionStore == nil {
		return nil
	}
	return b.chatSessionStore.Save(sessions)
}

// RunChatSession 自由对话多轮：在同一 chatSessionId 上续接完整 Agent transcript。
func (b *Bridge) RunChatSession(req ChatSessionRequest) (*RunResult, error) {
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		return nil, fmt.Errorf("消息不能为空")
	}
	history, err := b.chatTranscripts.Load(req.ChatSessionID, req.Bootstrap)
	if err != nil {
		return nil, err
	}
	userContent := msg
	if len(history) == 0 {
		userContent = b.formatUserMessage(msg)
	}
	initial := append(append([]query.Message(nil), history...), query.Message{
		Role:    query.RoleUser,
		Content: userContent,
	})
	return b.runAgentSession(initial, req.ChatSessionID, func(result query.Result) error {
		return b.chatTranscripts.Save(req.ChatSessionID, result.Messages)
	})
}
