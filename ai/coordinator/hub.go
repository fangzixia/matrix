package coordinator

import (
	"context"
	"strings"

	"matrix/ai/agent"
	"matrix/ai/query"
	"matrix/ai/stream"
	"time"

	agui "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// StreamHub 将 Worker 流式 AG-UI 事件与进度接到上层 UI。
type StreamHub struct {
	ThreadID  string
	RunID     string
	EmitUI    func(stream.Event)
	OnUpdate  func(agent.Snapshot)
	OnDone    func(agent.Snapshot)
	Registry  *agent.Registry
	Sidechain *agent.SidechainWriter
	Audit     query.AuditRecorder
	inner     query.StreamSink
}

// NewStreamHub 创建 Hub；publish/onUpdate/onDone 可为 nil（仅更新 Registry）。
func NewStreamHub(
	threadID, runID string,
	registry *agent.Registry,
	sidechain *agent.SidechainWriter,
	inner query.StreamSink,
	publish func(stream.Event),
	onUpdate, onDone func(agent.Snapshot),
) *StreamHub {
	return &StreamHub{
		ThreadID:  threadID,
		RunID:     runID,
		EmitUI:    publish,
		OnUpdate:  onUpdate,
		OnDone:    onDone,
		Registry:  registry,
		Sidechain: sidechain,
		inner:     inner,
	}
}

// WorkerSink 返回带 agent 元数据的 Sink，并驱动进度 / sidechain。
func (h *StreamHub) WorkerSink(agentID, parentAgentID, parentToolUseID string) query.StreamSink {
	if h == nil {
		return stream.NopSink{}
	}
	tag := stream.ScopeSink{
		Inner:           h,
		AgentID:         agentID,
		ParentAgentID:   parentAgentID,
		ParentToolUseID: parentToolUseID,
	}
	return stream.FuncSink(func(ctx context.Context, ev stream.Event) error {
		return tag.Publish(ctx, ev)
	})
}

// Publish 实现 stream.Sink：转发到 UI 并更新 Registry / sidechain。
func (h *StreamHub) Publish(_ context.Context, ev stream.Event) error {
	if h.inner != nil {
		_ = h.inner.Publish(context.Background(), ev)
	}
	if h.EmitUI != nil {
		h.EmitUI(ev)
	}
	h.recordSidechain(ev)
	h.applyProgress(ev)
	return nil
}

func (h *StreamHub) recordSidechain(ev stream.Event) {
	if h.Sidechain == nil {
		return
	}
	agentID := workerAgentID(ev)
	if agentID == "" {
		return
	}
	h.Sidechain.Append(agent.ID(agentID), map[string]any{
		"ts":    time.Now().UnixMilli(),
		"type":  stream.EventType(ev),
		"event": ev,
	})
}

func workerAgentID(ev stream.Event) string {
	if ev == nil {
		return ""
	}
	switch e := ev.(type) {
	case *agui.TextMessageStartEvent:
		return e.Name
	case *agui.TextMessageChunkEvent:
		if e.Name != nil {
			return *e.Name
		}
	case *agui.ActivitySnapshotEvent:
		if e.ActivityType == stream.ActivityTypeSubagent {
			if m, ok := e.Content.(map[string]any); ok {
				if id, _ := m["id"].(string); id != "" {
					return id
				}
			}
		}
	}
	return ""
}

const maxRecentActivities = 5

func (h *StreamHub) applyProgress(ev stream.Event) {
	if h.Registry == nil {
		return
	}
	agentID := workerAgentID(ev)
	if agentID == "" {
		return
	}
	id := agent.ID(agentID)
	updated := false
	h.Registry.Update(id, func(r *agent.Record) {
		switch ev.Type() {
		case agui.EventTypeStepStarted:
			if step, ok := ev.(*agui.StepStartedEvent); ok {
				turn := parseTurnStep(step.StepName)
				r.Progress.Turn = turn
				r.Progress.Transition = ""
				r.Progress.CurrentTool = ""
				r.Progress.Summary = query.TurnThinkingLabel(turn, "")
				r.Progress.LastActivity = r.Progress.Summary
				updated = true
			}
		case agui.EventTypeToolCallStart:
			if tc, ok := ev.(*agui.ToolCallStartEvent); ok {
				r.Progress.CurrentTool = tc.ToolCallName
				r.Progress.LastActivity = query.ToolActivityLabel(tc.ToolCallName, "started")
				r.Progress.RecentActivities = appendRecentActivity(r.Progress.RecentActivities, agent.ToolActivity{
					ToolName: tc.ToolCallName,
					Status:   "started",
				})
				updated = true
			}
		case agui.EventTypeCustom:
			if stream.IsToolOutputDelta(ev) {
				_, toolName, delta, ok := stream.ToolOutputDeltaFields(ev)
				if ok {
					r.Progress.CurrentTool = toolName
					r.Progress.LastActivity = query.ToolActivityLabel(toolName, "streaming")
					r.Progress.RecentActivities = appendRecentActivity(r.Progress.RecentActivities, agent.ToolActivity{
						ToolName: toolName,
						Status:   "streaming",
						Preview:  query.PreviewText(delta, 120),
					})
					updated = true
				}
			}
		case agui.EventTypeToolCallResult:
			if tr, ok := ev.(*agui.ToolCallResultEvent); ok {
				r.Progress.ToolUseCount++
				r.Progress.CurrentTool = ""
				r.Progress.LastActivity = query.TurnWithToolsLabel(r.Progress.Turn, r.Progress.ToolUseCount)
				r.Progress.RecentActivities = appendRecentActivity(r.Progress.RecentActivities, agent.ToolActivity{
					ToolName: tr.ToolCallID,
					Status:   "completed",
					Preview:  query.PreviewText(tr.Content, 120),
				})
				updated = true
			}
		case agui.EventTypeTextMessageContent:
			r.Progress.LastActivity = "整理回复中…"
			updated = true
		case agui.EventTypeRunFinished, agui.EventTypeRunError:
			r.Progress.LastActivity = "已完成"
			updated = true
		}
	})
	if updated && h.OnUpdate != nil {
		if rec := h.Registry.Get(id); rec != nil {
			h.OnUpdate(agent.ToSnapshot(rec))
			if h.inner != nil {
				_ = h.inner.Publish(context.Background(), stream.SubagentActivity(agent.ToSnapshot(rec)))
			}
		}
	}
}

func parseTurnStep(stepName string) int {
	stepName = strings.TrimPrefix(stepName, "turn-")
	n := 0
	for _, c := range stepName {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	if n < 1 {
		return 1
	}
	return n
}

func appendRecentActivity(items []agent.ToolActivity, act agent.ToolActivity) []agent.ToolActivity {
	items = append(items, act)
	if len(items) > maxRecentActivities {
		items = items[len(items)-maxRecentActivities:]
	}
	return items
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
			"description":        query.PreviewText(rec.Description, 200),
		})
	}
	snap := agent.ToSnapshot(rec)
	if h.inner != nil {
		_ = h.inner.Publish(context.Background(), stream.SubagentActivity(snap))
	}
	if h.OnUpdate != nil {
		h.OnUpdate(snap)
	}
}

// NotifyProgress 显式推送进度快照。
func (h *StreamHub) NotifyProgress(id agent.ID) {
	if h == nil || h.OnUpdate == nil || h.Registry == nil {
		return
	}
	if rec := h.Registry.Get(id); rec != nil {
		snap := agent.ToSnapshot(rec)
		if h.inner != nil {
			_ = h.inner.Publish(context.Background(), stream.SubagentActivity(snap))
		}
		h.OnUpdate(snap)
	}
}

// SidechainPath 返回某 Agent 的旁路 transcript 路径。
func (h *StreamHub) SidechainPath(id agent.ID) string {
	if h == nil || h.Sidechain == nil {
		return ""
	}
	return h.Sidechain.Path(id)
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
		if h.inner != nil {
			_ = h.inner.Publish(context.Background(), stream.SubagentActivity(snap))
		}
		if h.OnDone != nil {
			h.OnDone(snap)
		}
		if h.OnUpdate != nil {
			h.OnUpdate(snap)
		}
	}
}
