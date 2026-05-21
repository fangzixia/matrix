package desktop

import (
	"fmt"

	"matrix/internal/agent"
)

const (
	subAgentUpdateEvent = "subagent:update"
	subAgentDoneEvent   = "subagent:done"
)

// SubAgentSnapshot 为 Wails 绑定的子 Agent 状态 DTO（与 agent.Snapshot 字段一致）。
type SubAgentSnapshot struct {
	ID              string         `json:"id"`
	Description     string         `json:"description"`
	Status          string         `json:"status"`
	ParentAgentID   string         `json:"parent_agent_id,omitempty"`
	ParentToolUseID string         `json:"parent_tool_use_id,omitempty"`
	Progress        agent.Progress `json:"progress"`
	CreatedAt       int64          `json:"created_at"`
	SidechainPath   string         `json:"sidechain_path,omitempty"`
	AnswerPreview   string         `json:"answer_preview,omitempty"`
	TurnCount       int            `json:"turn_count,omitempty"`
}

func toSubAgentDTO(s agent.Snapshot) SubAgentSnapshot {
	return SubAgentSnapshot{
		ID:              s.ID,
		Description:     s.Description,
		Status:          string(s.Status),
		ParentAgentID:   s.ParentAgentID,
		ParentToolUseID: s.ParentToolUseID,
		Progress:        s.Progress,
		CreatedAt:       s.CreatedAt,
		SidechainPath:   s.SidechainPath,
		AnswerPreview:   s.AnswerPreview,
		TurnCount:       s.TurnCount,
	}
}

// ListSubAgents 返回当前会话子 Agent 列表。
func (b *Bridge) ListSubAgents() []SubAgentSnapshot {
	if b.subAgentRegistry == nil {
		return nil
	}
	recs := b.subAgentRegistry.List()
	out := make([]SubAgentSnapshot, 0, len(recs))
	for _, rec := range recs {
		out = append(out, toSubAgentDTO(agent.ToSnapshot(rec)))
	}
	return out
}

// GetSubAgent 按 id 查询子 Agent。
func (b *Bridge) GetSubAgent(id string) (*SubAgentSnapshot, error) {
	if b.subAgentRegistry == nil {
		return nil, fmt.Errorf("registry 未初始化")
	}
	rec := b.subAgentRegistry.Get(agent.ID(id))
	if rec == nil {
		return nil, fmt.Errorf("未找到子 Agent %q", id)
	}
	dto := toSubAgentDTO(agent.ToSnapshot(rec))
	return &dto, nil
}

// StopSubAgent 停止运行中的子 Agent（task_stop 的 Bridge 入口）。
func (b *Bridge) StopSubAgent(id, reason string) (string, error) {
	if reason == "" {
		reason = "用户从界面停止"
	}
	if b.workerRun == nil {
		return "", fmt.Errorf("RunControl 未初始化")
	}
	agentID := agent.ID(id)
	if b.subAgentRegistry.Get(agentID) == nil {
		return fmt.Sprintf("子 Agent %q 不存在或已完成", id), nil
	}
	if !b.workerRun.Stop(agentID) {
		return fmt.Sprintf("子 Agent %q 未在运行", id), nil
	}
	b.subAgentRegistry.Update(agentID, func(r *agent.Record) {
		r.Status = agent.StatusStopped
	})
	if b.sessions != nil && b.sessions.hub != nil {
		b.sessions.hub.NotifyDone(agentID)
	}
	return fmt.Sprintf("已停止子 Agent %s：%s", id, reason), nil
}

// ReadSubAgentTranscript 读取 sidechain JSONL 末尾内容。
func (b *Bridge) ReadSubAgentTranscript(id string, maxLines int) (string, error) {
	if b.sessions == nil || b.sessions.sidechain == nil {
		return "", fmt.Errorf("当前无活动会话或未启用 sidechain")
	}
	return b.sessions.sidechain.ReadTail(agent.ID(id), maxLines)
}
