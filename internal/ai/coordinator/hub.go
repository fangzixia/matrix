package coordinator

import (
	"context"
	"matrix/internal/ai/activity"
	"matrix/internal/ai/agent"
	"matrix/internal/ai/audit"
	"matrix/internal/ai/query"
	"matrix/internal/ai/stream"
	"sync"
	"time"
)

// SubAgentStreamHub 将 Worker 流式消息、进度与嵌套 Async 通道接到上层（如 desktop Bridge）。
type SubAgentStreamHub interface {
	WorkerSink(agentID, parentAgentID, parentToolUseID string) query.StreamSink
	EnsureWorkerAsync(id agent.ID) *AsyncSupport
	NotifySpawn(rec *agent.Record)
	NotifyProgress(id agent.ID)
	NotifyDone(id agent.ID)
}

// StreamHub 实现 SubAgentStreamHub：打标签推流、更新 Registry、写 sidechain、发嵌套 Async。
type StreamHub struct {
	SessionID string
	EmitUI    func(stream.Message)
	OnUpdate  func(agent.Snapshot)
	OnDone    func(agent.Snapshot)
	Registry  *agent.Registry
	Sidechain *agent.SidechainWriter
	Audit     *audit.Writer
	mu        sync.Mutex
	nested    map[agent.ID]*AsyncSupport
	inner     query.StreamSink
}

// NewStreamHub 创建 Hub；publish/onUpdate/onDone 可为 nil（仅更新 Registry）。
func NewStreamHub(
	sessionID string,
	registry *agent.Registry,
	sidechain *agent.SidechainWriter,
	inner query.StreamSink,
	publish func(stream.Message),
	onUpdate, onDone func(agent.Snapshot),
) *StreamHub {
	return &StreamHub{
		SessionID: sessionID,
		EmitUI:    publish,
		OnUpdate:  onUpdate,
		OnDone:    onDone,
		Registry:  registry,
		Sidechain: sidechain,
		nested:    make(map[agent.ID]*AsyncSupport),
		inner:     inner,
	}
}

// WorkerSink 返回带 agent 元数据的 Sink，并驱动进度 / sidechain。
func (h *StreamHub) WorkerSink(agentID, parentAgentID, parentToolUseID string) query.StreamSink {
	if h == nil {
		return stream.NopSink{}
	}
	tag := stream.TagSink{
		Inner:           h,
		AgentID:         agentID,
		ParentAgentID:   parentAgentID,
		ParentToolUseID: parentToolUseID,
	}
	return stream.FuncSink(func(ctx context.Context, msg stream.Message) error {
		return tag.Publish(ctx, msg)
	})
}

// Publish 实现 stream.Sink：转发到 UI 并更新 Registry / sidechain。
func (h *StreamHub) Publish(_ context.Context, msg stream.Message) error {
	if h.inner != nil {
		_ = h.inner.Publish(context.Background(), msg)
	}
	if h.EmitUI != nil {
		h.EmitUI(msg)
	}
	h.recordSidechain(msg)
	h.applyProgress(msg)
	return nil
}

// recordSidechain 将 Worker 旁路 transcript 写入 sidechain 文件。
func (h *StreamHub) recordSidechain(msg stream.Message) {
	if h.Sidechain == nil || msg.AgentID == "" {
		return
	}
	h.Sidechain.Append(agent.ID(msg.AgentID), map[string]any{
		"ts":   time.Now().UnixMilli(),
		"type": msg.Type,
		"msg":  msg,
	})
}

const maxRecentActivities = 5

// applyProgress 根据流式消息更新 Agent 进度快照。
func (h *StreamHub) applyProgress(msg stream.Message) {
	if h.Registry == nil || msg.AgentID == "" {
		return
	}
	id := agent.ID(msg.AgentID)
	updated := false
	h.Registry.Update(id, func(r *agent.Record) {
		switch msg.Type {
		case stream.TypeProgress:
			if msg.Data == nil {
				return
			}
			switch msg.Data.Type {
			case stream.DataTurnProgress:
				r.Progress.Turn = msg.Data.Turn
				r.Progress.Transition = msg.Data.Transition
				r.Progress.CurrentTool = ""
				r.Progress.Summary = activity.TurnSummary(msg.Data.Turn)
				r.Progress.LastActivity = activity.TurnThinkingLabel(msg.Data.Turn, msg.Data.Transition)
				updated = true
			case stream.DataToolProgress, stream.DataMCPProgress:
				if msg.Data.ToolName != "" {
					r.Progress.CurrentTool = msg.Data.ToolName
					r.Progress.LastActivity = activity.ToolActivityLabel(msg.Data.ToolName, msg.Data.Status)
					if msg.Data.Status == "started" || msg.Data.Status == "streaming" || msg.Data.Status == "completed" || msg.Data.Status == "failed" {
						preview := msg.Data.Message
						if msg.Data.Type == stream.DataToolOutputDelta && msg.Data.Delta != "" {
							preview = msg.Data.Delta
						}
						r.Progress.RecentActivities = appendRecentActivity(r.Progress.RecentActivities, agent.ToolActivity{
							ToolName: msg.Data.ToolName,
							Status:   msg.Data.Status,
							Preview:  audit.Preview(preview, 120),
						})
					}
				}
				if msg.Data.Status == "completed" {
					r.Progress.ToolUseCount++
					r.Progress.CurrentTool = ""
					r.Progress.LastActivity = activity.TurnWithToolsLabel(r.Progress.Turn, r.Progress.ToolUseCount)
				} else if msg.Data.Status == "failed" {
					r.Progress.CurrentTool = ""
				}
				updated = true
			case stream.DataToolOutputDelta:
				if msg.Data.ToolName != "" {
					r.Progress.CurrentTool = msg.Data.ToolName
					r.Progress.LastActivity = activity.ToolActivityLabel(msg.Data.ToolName, "streaming")
					r.Progress.RecentActivities = appendRecentActivity(r.Progress.RecentActivities, agent.ToolActivity{
						ToolName: msg.Data.ToolName,
						Status:   "streaming",
						Preview:  audit.Preview(msg.Data.Delta, 120),
					})
				}
				updated = true
			}
		case stream.TypeAssistant:
			if msg.Assistant != nil {
				r.Progress.LastActivity = "整理回复中…"
				updated = true
			}
		case stream.TypeResult:
			r.Progress.LastActivity = "已完成"
			updated = true
		}
	})
	if updated && h.OnUpdate != nil {
		if rec := h.Registry.Get(id); rec != nil {
			h.OnUpdate(agent.ToSnapshot(rec))
		}
	}
}

func appendRecentActivity(items []agent.ToolActivity, act agent.ToolActivity) []agent.ToolActivity {
	items = append(items, act)
	if len(items) > maxRecentActivities {
		items = items[len(items)-maxRecentActivities:]
	}
	return items
}

// EnsureWorkerAsync 为 Worker（及嵌套子 Worker）分配独立 AsyncSupport。
func (h *StreamHub) EnsureWorkerAsync(id agent.ID) *AsyncSupport {
	if h == nil {
		return NewAsyncSupport()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if a, ok := h.nested[id]; ok {
		return a
	}
	a := NewAsyncSupport()
	h.nested[id] = a
	return a
}

// NotifySpawn 注册后通知 UI。
func (h *StreamHub) NotifySpawn(rec *agent.Record) {
	if rec == nil {
		return
	}
	if h.Audit != nil {
		h.Audit.Emit("subagent.spawn", 0, "coordinator", map[string]any{
			"agent_id":           string(rec.ID),
			"parent_agent_id":    string(rec.ParentAgentID),
			"parent_tool_use_id": rec.ParentToolUseID,
			"description":        audit.Preview(rec.Description, 200),
		})
	}
	if h.OnUpdate != nil {
		h.OnUpdate(agent.ToSnapshot(rec))
	}
}

// NotifyProgress 显式推送进度快照（实现 SubAgentStreamHub）。
func (h *StreamHub) NotifyProgress(id agent.ID) {
	if h == nil || h.OnUpdate == nil || h.Registry == nil {
		return
	}
	if rec := h.Registry.Get(id); rec != nil {
		h.OnUpdate(agent.ToSnapshot(rec))
	}
}

// SidechainPath 返回某 Agent 的旁路 transcript 路径。
func SidechainPath(hub SubAgentStreamHub, id agent.ID) string {
	if hub == nil {
		return ""
	}
	if h, ok := hub.(*StreamHub); ok && h.Sidechain != nil {
		return h.Sidechain.Path(id)
	}
	return ""
}

// NotifyDone Worker 结束时推送。
func (h *StreamHub) NotifyDone(id agent.ID) {
	if h == nil {
		return
	}
	if rec := h.Registry.Get(id); rec != nil {
		stopReason := ""
		turnCount := 0
		if rec.Result != nil {
			stopReason = string(rec.Result.StopReason)
			turnCount = rec.Result.TurnCount
		}
		if h.Audit != nil {
			h.Audit.Emit("subagent.done", 0, "coordinator", map[string]any{
				"agent_id":    string(id),
				"stop_reason": stopReason,
				"turn_count":  turnCount,
				"sidechain":   rec.SidechainPath,
			})
		}
		snap := agent.ToSnapshot(rec)
		if h.OnDone != nil {
			h.OnDone(snap)
		}
		if h.OnUpdate != nil {
			h.OnUpdate(snap)
		}
	}
}
