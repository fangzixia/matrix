package desktop

import (
	"encoding/json"

	"matrix/internal/query"
)

// ChatTurn 是前端传入的一轮对话摘要（仅 user/assistant 文本）。
// 在 Agent transcript 磁盘缓存为空时，由对话历史降级恢复（bootstrap）。
type ChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatMessage 是对话历史中的一条展示消息。
type ChatMessage struct {
	Role            string          `json:"role"`
	Content         string          `json:"content"`
	Time            string          `json:"time,omitempty"`
	SessionSnapshot json.RawMessage `json:"sessionSnapshot,omitempty"`
}

// ChatSession 是对话历史条目（标题 + 展示消息；Agent transcript 另存 transcripts/）。
type ChatSession struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Messages []ChatMessage `json:"messages"`
}

// ChatSessionRequest 自由对话多轮 Wails 请求。
type ChatSessionRequest struct {
	ChatSessionID string     `json:"chatSessionId"`
	Message       string     `json:"message"`
	Bootstrap     []ChatTurn `json:"bootstrap,omitempty"`
}

// chatTurnsToMessages 将 bootstrap 轮次转为 Agent 消息（不含 tool/thinking）。
func chatTurnsToMessages(turns []ChatTurn) []query.Message {
	out := make([]query.Message, 0, len(turns))
	for _, t := range turns {
		role := query.RoleUser
		switch t.Role {
		case "assistant":
			role = query.RoleAssistant
		case "user", "":
			role = query.RoleUser
		default:
			continue
		}
		if t.Content == "" {
			continue
		}
		out = append(out, query.Message{Role: role, Content: t.Content})
	}
	return out
}
