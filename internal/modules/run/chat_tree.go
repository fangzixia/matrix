package run

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"matrix/internal/ai/query"

	"github.com/google/uuid"
)

const chatMessagesVersion = 2

// ChatMessageNode 是会话消息树中的单条节点。
type ChatMessageNode struct {
	ID          string                    `json:"id"`
	ParentID    *string                   `json:"parent_id"`
	Role        string                    `json:"role"`
	Content     string                    `json:"content"`
	Attachments []query.MessageAttachment `json:"attachments,omitempty"`
	RunID       string                    `json:"run_id,omitempty"`
	CreatedAt   string                    `json:"created_at,omitempty"`
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

// NodesToQueryMessages 将消息节点转为 query.Message 列表。
func NodesToQueryMessages(nodes []ChatMessageNode) []query.Message {
	out := make([]query.Message, 0, len(nodes))
	for _, n := range nodes {
		role := query.RoleUser
		switch strings.ToLower(n.Role) {
		case "assistant":
			role = query.RoleAssistant
		case "system":
			role = query.RoleSystem
		case "user":
			role = query.RoleUser
		default:
			role = query.Role(n.Role)
		}
		out = append(out, query.Message{
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

// ValidateSessionMessages 校验消息树结构（唯一 id、合法 parent 引用）。
func ValidateSessionMessages(sm SessionMessages) error {
	ids := make(map[string]struct{}, len(sm.Nodes))
	for _, n := range sm.Nodes {
		if strings.TrimSpace(n.ID) == "" {
			return errors.New("消息 id 不能为空")
		}
		if _, dup := ids[n.ID]; dup {
			return fmt.Errorf("重复的消息 id：%s", n.ID)
		}
		ids[n.ID] = struct{}{}
	}
	for _, n := range sm.Nodes {
		if n.ParentID == nil || *n.ParentID == "" {
			continue
		}
		if _, ok := ids[*n.ParentID]; !ok {
			return fmt.Errorf("节点 %s 的 parent_id 无效：%s", n.ID, *n.ParentID)
		}
	}
	if sm.ActiveLeafID != "" {
		if _, ok := ids[sm.ActiveLeafID]; !ok {
			return fmt.Errorf("active_leaf_id 不存在：%s", sm.ActiveLeafID)
		}
	}
	return nil
}

// AppendNode 追加节点并将 active_leaf_id 更新为新节点。
func AppendNode(sm SessionMessages, node ChatMessageNode) SessionMessages {
	if sm.Version == 0 {
		sm.Version = chatMessagesVersion
	}
	sm.Nodes = append(sm.Nodes, node)
	sm.ActiveLeafID = node.ID
	return sm
}

// NewUserNode 创建待持久化的用户消息节点。
func NewUserNode(parentID string, content string, attachments []query.MessageAttachment) ChatMessageNode {
	node := ChatMessageNode{
		ID:          uuid.New().String(),
		Role:        "user",
		Content:     content,
		Attachments: attachments,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	parentID = strings.TrimSpace(parentID)
	if parentID != "" {
		node.ParentID = &parentID
	}
	return node
}

// HistoryForParent 返回挂到 parentID 时应传给 LLM 的历史消息。
func HistoryForParent(sm SessionMessages, parentID string) ([]query.Message, error) {
	ancestors, err := WalkAncestors(sm, parentID)
	if err != nil {
		return nil, err
	}
	return NodesToQueryMessages(ancestors), nil
}

// UpsertNode 按 id 更新或追加节点；若节点已存在则替换，否则追加。
func UpsertNode(sm SessionMessages, node ChatMessageNode) SessionMessages {
	for i := range sm.Nodes {
		if sm.Nodes[i].ID == node.ID {
			sm.Nodes[i] = node
			sm.ActiveLeafID = node.ID
			return sm
		}
	}
	return AppendNode(sm, node)
}
