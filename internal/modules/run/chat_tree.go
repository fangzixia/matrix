package run

import (
	"fmt"
	"strings"

	ai "matrix/ai/sdk"
)

const chatMessagesVersion = 2

// ChatMessageNode 是会话消息树中的单条节点。
type ChatMessageNode struct {
	ID          string                 `json:"id"`
	ParentID    *string                `json:"parent_id"`
	Role        string                 `json:"role"`
	Content     string                 `json:"content"`
	Attachments []ai.MessageAttachment `json:"attachments,omitempty"`
	RunID       string                 `json:"run_id,omitempty"`
	CreatedAt   string                 `json:"created_at,omitempty"`
}

// SessionMessages 是会话消息树（内存结构）。
type SessionMessages struct {
	Version      int               `json:"version"`
	ActiveLeafID string            `json:"active_leaf_id,omitempty"`
	Nodes        []ChatMessageNode `json:"nodes"`
}

// nodeIndex 构建 id -> node 索引。
func nodeIndex(nodes []ChatMessageNode) map[string]ChatMessageNode {
	idx := make(map[string]ChatMessageNode, len(nodes))
	for _, n := range nodes {
		idx[n.ID] = n
	}
	return idx
}

// WalkAncestors 从 nodeID 沿 parent_id 回溯至根，返回根到 nodeID 的顺序（含 nodeID）。
// nodeID 为空时返回 nil，表示会话首条消息之前无历史。
func WalkAncestors(sm SessionMessages, nodeID string) ([]ChatMessageNode, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil, nil
	}
	idx := nodeIndex(sm.Nodes)
	var chain []ChatMessageNode
	seen := make(map[string]struct{})
	cur := nodeID
	for cur != "" {
		if _, ok := seen[cur]; ok {
			return nil, fmt.Errorf("消息树存在环：节点 %s", cur)
		}
		seen[cur] = struct{}{}
		n, ok := idx[cur]
		if !ok {
			return nil, fmt.Errorf("节点不存在：%s", cur)
		}
		chain = append(chain, n)
		if n.ParentID == nil || *n.ParentID == "" {
			break
		}
		cur = *n.ParentID
	}
	// reverse to root -> leaf
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// NodesToQueryMessages 将消息节点转为 ai.Message 列表。
func NodesToQueryMessages(nodes []ChatMessageNode) []ai.Message {
	out := make([]ai.Message, 0, len(nodes))
	for _, n := range nodes {
		role := ai.RoleUser
		switch strings.ToLower(n.Role) {
		case "assistant":
			role = ai.RoleAssistant
		case "system":
			role = ai.RoleSystem
		case "user":
			role = ai.RoleUser
		default:
			role = ai.Role(n.Role)
		}
		out = append(out, ai.Message{
			Role:        role,
			Content:     n.Content,
			Attachments: n.Attachments,
		})
	}
	return out
}

// ValidateParent 校验 parent_id 是否属于当前会话（空表示首条消息）。
func ValidateParent(sm SessionMessages, parentID string) error {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return nil
	}
	idx := nodeIndex(sm.Nodes)
	if _, ok := idx[parentID]; !ok {
		return fmt.Errorf("parent_id 不存在：%s", parentID)
	}
	return nil
}

// HistoryForParent 返回挂到 parentID 时应传给 LLM 的历史消息。
func HistoryForParent(sm SessionMessages, parentID string) ([]ai.Message, error) {
	ancestors, err := WalkAncestors(sm, parentID)
	if err != nil {
		return nil, err
	}
	return NodesToQueryMessages(ancestors), nil
}
