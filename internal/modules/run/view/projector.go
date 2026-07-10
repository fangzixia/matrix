package view

import (
	"fmt"
	"matrix/internal/ai/activity"
	"matrix/internal/ai/agent"
	"matrix/internal/ai/stream"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxLiveOutputRunes = 8192

// Projector 将内部 stream.Message 投影为 RunViewState 与 AG-UI 事件。
type Projector struct {
	runID     string
	projectID string
	state     RunViewState
	parse     parserState
	messageID string
	lastSnap  time.Time
}

type parserState struct {
	coordinatorTurn           *TurnView
	workerCurrentTurn         map[string]*TurnView
	workerParentToolID        map[string]string // agentID → 触发 agent 工具的 tool_use id
	parentToolCoordinatorTurn map[string]int    // parentToolUseID → 协调者轮次号
}

// NewProjector 创建 Run 视图投影器。
func NewProjector(runID, projectID string) *Projector {
	s := NewRunViewState(runID)
	return &Projector{
		runID:     runID,
		projectID: projectID,
		state:     s,
		parse: parserState{
			workerCurrentTurn:         make(map[string]*TurnView),
			workerParentToolID:        make(map[string]string),
			parentToolCoordinatorTurn: make(map[string]int),
		},
	}
}

// State 返回当前视图状态副本。
func (p *Projector) State() RunViewState {
	return cloneState(p.state)
}

// Apply 处理一条内部消息并返回待发出的 AG-UI 事件。
func (p *Projector) Apply(msg stream.Message) []Envelope {
	var out []Envelope
	now := time.Now().UnixMilli()

	switch msg.Type {
	case stream.TypeProgress:
		out = append(out, p.applyProgress(msg, now)...)
	case stream.TypeStreamEvent:
		out = append(out, p.applyStreamEvent(msg, now)...)
	case stream.TypeAssistant:
		out = append(out, p.applyAssistant(msg)...)
	case stream.TypeResult:
		p.applyResult(msg)
	case stream.TypeSubAgentUpdate, stream.TypeSubAgentDone:
		p.applySubagentSnapshot(msg.Snapshot)
		out = append(out, p.emitActivitySnapshot(now))
	}

	p.syncReplyText()
	if p.shouldEmitSnapshot() {
		out = append(out, p.emitStateSnapshot(now))
		p.lastSnap = time.Now()
	}
	return out
}

// OnSubagent 由 coordinator 直接更新子 Agent 快照。
func (p *Projector) OnSubagent(snap agent.Snapshot) []Envelope {
	p.setSubagentFromSnapshot(snap)
	p.state.StatusLabel = subagentStatusLabel(snap)
	now := time.Now().UnixMilli()
	return []Envelope{
		p.emitActivitySnapshot(now),
		p.emitStateSnapshot(now),
	}
}

func (p *Projector) applyProgress(msg stream.Message, now int64) []Envelope {
	if msg.Data == nil {
		return nil
	}
	data := msg.Data
	var out []Envelope

	switch data.Type {
	case stream.DataTurnProgress:
		turnNum := data.Turn
		if turnNum < 1 {
			turnNum = 1
		}
		if msg.Scope == stream.ScopeWorker {
			p.ensureWorkerTurn(msg, turnNum, data.Summary)
		} else {
			p.ensureCoordinatorTurn(turnNum, data.Summary)
		}
		p.state.StatusLabel = activity.TurnThinkingLabel(turnNum, data.Transition)
		out = append(out, p.emitActivitySnapshot(now))

	case stream.DataToolOutputDelta:
		turn := p.resolveToolTurn(msg, msg.ToolUseID)
		if turn == nil {
			turn = p.activeTurn(msg)
		}
		if turn != nil {
			p.appendToolOutputDelta(turn, msg.ToolUseID, data.ToolName, data.Delta)
			p.refreshTurnSummary(turn)
		}
		if data.ToolName != "" {
			p.state.StatusLabel = activity.ToolActivityLabel(data.ToolName, "streaming")
		}

	case stream.DataToolProgress, stream.DataMCPProgress:
		if data.Status == "input_streaming" && data.Delta != "" {
			toolUseID := msg.ToolUseID
			if toolUseID == "" {
				toolUseID = fmt.Sprintf("input-%s", data.ToolName)
			}
			turn := p.resolveToolTurn(msg, toolUseID)
			if turn == nil {
				turn = p.activeTurn(msg)
			}
			if turn == nil {
				return out
			}
			if !isProvisionalToolID(toolUseID) {
				p.reconcileProvisionalTools(turn, data.ToolName, toolUseID)
			}
			tool := p.upsertToolInTurn(turn, toolUseID, data.ToolName, "loading", data.ServerName, "", 0)
			if tool != nil {
				tool.Preview = tool.Preview + data.Delta
			}
			p.refreshTurnSummary(turn)
			out = append(out, Envelope{
				Type: EventTOOLCallArgs, RunID: p.runID, Timestamp: now,
				Payload: ToolCallArgsPayload{ToolCallID: toolUseID, Delta: data.Delta},
			})
		} else if data.Status == "started" {
			toolUseID := msg.ToolUseID
			if toolUseID == "" {
				return out
			}
			turn := p.resolveToolTurn(msg, toolUseID)
			if turn == nil {
				turn = p.activeTurn(msg)
			}
			if turn == nil {
				return out
			}
			p.reconcileProvisionalTools(turn, data.ToolName, toolUseID)
			p.upsertToolInTurn(turn, toolUseID, data.ToolName, "loading", data.ServerName, data.Message, 0)
			p.refreshTurnSummary(turn)
			p.rememberCoordinatorToolTurn(msg, toolUseID)
			p.state.StatusLabel = activity.ToolActivityLabel(data.ToolName, "started")
			out = append(out, Envelope{
				Type: EventTOOLCallStart, RunID: p.runID, Timestamp: now,
				Payload: ToolCallStartPayload{
					ToolCallID: toolUseID, ToolCallName: data.ToolName, ServerName: data.ServerName,
				},
			})
		} else if data.Status == "completed" || data.Status == "failed" {
			toolUseID := msg.ToolUseID
			if toolUseID == "" {
				return out
			}
			turn := p.resolveToolTurn(msg, toolUseID)
			if turn == nil {
				turn = p.activeTurn(msg)
			}
			if turn == nil {
				return out
			}
			status := mapToolStatus(data.Status)
			if data.ToolName == "" {
				if existing, _ := p.findToolGlobal(toolUseID); existing != nil {
					data.ToolName = existing.ToolCallName
				}
			}
			p.reconcileProvisionalTools(turn, data.ToolName, toolUseID)
			tool := p.upsertToolInTurn(turn, toolUseID, data.ToolName, status, data.ServerName, data.Message, data.ElapsedTimeMs)
			if tool != nil {
				tool.OutputStreaming = false
			}
			p.refreshTurnSummary(turn)
			p.lastSnap = time.Time{} // 工具终态立即触发快照，避免 500ms 节流 + SSE 轮询延迟
			if toolUseID != "" {
				out = append(out,
					Envelope{Type: EventTOOLCallEnd, RunID: p.runID, Timestamp: now,
						Payload: ToolCallEndPayload{ToolCallID: toolUseID}},
					Envelope{Type: EventTOOLCallResult, RunID: p.runID, Timestamp: now,
						Payload: ToolCallResultPayload{
							ToolCallID: toolUseID, Status: data.Status,
							Preview: truncateStr(data.Message, 500), ElapsedMs: data.ElapsedTimeMs,
							LogURL: p.toolLogURL(toolUseID),
						}},
				)
			}
			p.state.StatusLabel = activity.TurnWithToolsLabel(turn.Turn, countFinishedTools(turn.Tools))
		}
	}
	return out
}

func (p *Projector) applyStreamEvent(msg stream.Message, now int64) []Envelope {
	if msg.Event == nil {
		return nil
	}
	turn := p.activeTurn(msg)
	if turn == nil {
		return nil
	}
	ev := msg.Event
	var out []Envelope

	switch ev.Type {
	case stream.EventMessageStart:
		turn.MessageStreaming = true
		turn.ThinkingStreaming = true
		if p.messageID == "" {
			p.messageID = uuid.NewString()
		}
		out = append(out, Envelope{
			Type: EventTEXTMessageStart, RunID: p.runID, Timestamp: now,
			Payload: TextMessageStartPayload{
				MessageID: p.messageID, Scope: string(msg.Scope), AgentID: msg.AgentID,
			},
		})
	case stream.EventMessageStop:
		turn.MessageStreaming = false
		turn.ThinkingStreaming = false
		if p.messageID != "" {
			out = append(out, Envelope{
				Type: EventTEXTMessageEnd, RunID: p.runID, Timestamp: now,
				Payload: TextMessageEndPayload{MessageID: p.messageID},
			})
			out = append(out, Envelope{
				Type: EventREASONINGMessageEnd, RunID: p.runID, Timestamp: now,
				Payload: TextMessageEndPayload{MessageID: p.messageID},
			})
		}
	case stream.EventContentBlockDelta:
		if ev.Delta == nil {
			return out
		}
		mid := p.messageID
		if mid == "" {
			mid = uuid.NewString()
			p.messageID = mid
		}
		if ev.Delta.Type == stream.DeltaThinking && ev.Delta.Thinking != "" {
			turn.Thinking += ev.Delta.Thinking
			p.refreshTurnSummary(turn)
			out = append(out, Envelope{
				Type: EventREASONINGMessageContent, RunID: p.runID, Timestamp: now,
				Payload: TextDeltaPayload{MessageID: mid, Delta: ev.Delta.Thinking},
			})
		}
		if ev.Delta.Type == stream.DeltaText && ev.Delta.Text != "" {
			turn.Message += ev.Delta.Text
			p.refreshTurnSummary(turn)
			out = append(out, Envelope{
				Type: EventTEXTMessageContent, RunID: p.runID, Timestamp: now,
				Payload: TextDeltaPayload{MessageID: mid, Delta: ev.Delta.Text},
			})
		}
	}
	return out
}

func (p *Projector) applyAssistant(msg stream.Message) []Envelope {
	if msg.Assistant == nil {
		return nil
	}
	turn := p.activeTurn(msg)
	if turn == nil {
		return nil
	}
	var thinking, text string
	for _, block := range msg.Assistant.Content {
		if block.Type == "thinking" && block.Thinking != "" {
			thinking += block.Thinking
		}
		if block.Type == "text" && block.Text != "" {
			text += block.Text
		}
	}
	if thinking != "" {
		turn.Thinking = thinking
	}
	if text != "" {
		turn.Message = text
	}
	turn.ThinkingStreaming = false
	turn.MessageStreaming = false
	p.refreshTurnSummary(turn)
	return nil
}

func (p *Projector) applyResult(msg stream.Message) {
	result := &ResultView{
		Subtype:    msg.Subtype,
		Output:     msg.Output,
		IsError:    msg.IsError,
		Error:      msg.ErrorMessage,
		NumTurns:   msg.NumTurns,
		DurationMs: msg.DurationMs,
		StopReason: msg.StopReason,
	}
	if msg.Output != "" && !msg.IsError {
		p.state.Result = result
	} else if msg.IsError || msg.Subtype == stream.ResultError || msg.Subtype == stream.ResultErrorMaxTurns {
		p.state.Result = result
		if msg.ErrorMessage != "" {
			p.state.Error = FormatUserRunError(msg.ErrorMessage)
		}
	} else if p.state.Result == nil {
		p.state.Result = result
	}
	p.finalizeStaleTools(!msg.IsError && msg.Subtype != stream.ResultError && msg.Subtype != stream.ResultErrorMaxTurns)
}

// finalizeStaleTools 在 Run 结束时清理仍卡在 loading 的工具条目（投影遗漏时的兜底）。
func (p *Projector) finalizeStaleTools(runSucceeded bool) {
	var walk func(turn *TurnView)
	walk = func(turn *TurnView) {
		for i := range turn.Tools {
			t := &turn.Tools[i]
			if t.Status == "loading" {
				t.OutputStreaming = false
				if runSucceeded {
					t.Status = "success"
				}
			}
			for j := range t.WorkerTurns {
				walk(&t.WorkerTurns[j])
			}
		}
	}
	for i := range p.state.Turns {
		walk(&p.state.Turns[i])
	}
}

func (p *Projector) applySubagentSnapshot(snapshot any) {
	if snapshot == nil {
		return
	}
	switch v := snapshot.(type) {
	case agent.Snapshot:
		p.setSubagentFromSnapshot(v)
	case map[string]any:
		p.setSubagentFromMap(v)
	}
}

func (p *Projector) setSubagentFromSnapshot(snap agent.Snapshot) {
	if snap.ID == "" {
		return
	}
	if p.state.Subagents == nil {
		p.state.Subagents = make(map[string]SubagentView)
	}
	prog := map[string]any{
		"turn":           snap.Progress.Turn,
		"transition":     snap.Progress.Transition,
		"summary":        snap.Progress.Summary,
		"current_tool":   snap.Progress.CurrentTool,
		"tool_use_count": snap.Progress.ToolUseCount,
		"last_activity":  snap.Progress.LastActivity,
	}
	p.state.Subagents[snap.ID] = SubagentView{
		ID: snap.ID, Description: snap.Description, Status: string(snap.Status),
		ParentAgentID: snap.ParentAgentID, ParentToolUseID: snap.ParentToolUseID,
		Progress: prog, CreatedAt: snap.CreatedAt, SidechainPath: snap.SidechainPath,
		AnswerPreview: snap.AnswerPreview, TurnCount: snap.TurnCount,
	}
	if snap.ParentToolUseID != "" {
		p.parse.workerParentToolID[snap.ID] = snap.ParentToolUseID
		if p.parse.coordinatorTurn != nil {
			p.parse.parentToolCoordinatorTurn[snap.ParentToolUseID] = p.parse.coordinatorTurn.Turn
		}
	}
}

func (p *Projector) setSubagentFromMap(m map[string]any) {
	id, _ := m["id"].(string)
	if id == "" {
		return
	}
	if p.state.Subagents == nil {
		p.state.Subagents = make(map[string]SubagentView)
	}
	sv := SubagentView{ID: id}
	if v, ok := m["description"].(string); ok {
		sv.Description = v
	}
	if v, ok := m["status"].(string); ok {
		sv.Status = v
	}
	if v, ok := m["parent_agent_id"].(string); ok {
		sv.ParentAgentID = v
	}
	if v, ok := m["parent_tool_use_id"].(string); ok {
		sv.ParentToolUseID = v
	}
	if sv.ParentToolUseID != "" {
		p.parse.workerParentToolID[id] = sv.ParentToolUseID
		if p.parse.coordinatorTurn != nil {
			p.parse.parentToolCoordinatorTurn[sv.ParentToolUseID] = p.parse.coordinatorTurn.Turn
		}
	}
	if v, ok := m["progress"].(map[string]any); ok {
		sv.Progress = v
	}
	p.state.Subagents[id] = sv
}

func (p *Projector) syncReplyText() {
	if p.state.Result != nil && p.state.Result.Output != "" && !p.state.Result.IsError {
		p.state.ReplyText = strings.TrimSpace(p.state.Result.Output)
		return
	}
	for i := len(p.state.Turns) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(p.state.Turns[i].Message); t != "" {
			p.state.ReplyText = t
			return
		}
	}
}

func (p *Projector) shouldEmitSnapshot() bool {
	return time.Since(p.lastSnap) >= 500*time.Millisecond
}

func (p *Projector) emitStateSnapshot(now int64) Envelope {
	return Envelope{
		Type: EventSTATESnapshot, RunID: p.runID, Timestamp: now,
		Payload: cloneState(p.state),
	}
}

func (p *Projector) emitActivitySnapshot(now int64) Envelope {
	return Envelope{
		Type: EventACTIVITYSnapshot, RunID: p.runID, Timestamp: now,
		Payload: ActivitySnapshotPayload{
			Subagents: p.state.Subagents, StatusLabel: p.state.StatusLabel,
		},
	}
}

func (p *Projector) toolLogURL(toolUseID string) string {
	if p.projectID == "" || toolUseID == "" {
		return ""
	}
	return fmt.Sprintf("/api/projects/%s/runs/%s/tools/%s/log", p.projectID, p.runID, toolUseID)
}

// --- parser helpers (ported from runActivityParser.ts) ---

func (p *Projector) ensureCoordinatorTurn(turnNum int, summary string) *TurnView {
	if p.parse.coordinatorTurn != nil && p.parse.coordinatorTurn.Turn == turnNum {
		if summary != "" {
			p.parse.coordinatorTurn.Summary = summary
		}
		return p.parse.coordinatorTurn
	}
	turn := TurnView{
		Key: fmt.Sprintf("coord-turn-%d", turnNum), Turn: turnNum, Scope: "coordinator",
		Summary: summary, Tools: []ToolView{},
	}
	p.state.Turns = append(p.state.Turns, turn)
	p.parse.coordinatorTurn = &p.state.Turns[len(p.state.Turns)-1]
	return p.parse.coordinatorTurn
}

func (p *Projector) ensureWorkerTurn(msg stream.Message, turnNum int, summary string) *TurnView {
	parentID := p.resolveParentToolUseID(msg)
	tool := p.findTool(parentID)
	wk := p.workerMapKeyFrom(msg)
	if tool == nil {
		if parentID != "" {
			tool = p.ensureParentAgentToolPlaceholder(parentID)
		}
		if tool == nil {
			if wk != "" {
				if existing, ok := p.parse.workerCurrentTurn[wk]; ok {
					if summary != "" {
						existing.Summary = summary
					}
					return existing
				}
			}
			return nil
		}
	}
	if wk == "" {
		return nil
	}
	if existing, ok := p.parse.workerCurrentTurn[wk]; ok && existing.Turn == turnNum {
		if summary != "" {
			existing.Summary = summary
		}
		return existing
	}
	turn := TurnView{
		Key:  fmt.Sprintf("worker-%s-turn-%d", msg.AgentID, turnNum),
		Turn: turnNum, Scope: "worker", AgentID: msg.AgentID,
		ParentToolUseID: parentID, Summary: summary, Tools: []ToolView{},
	}
	tool.WorkerTurns = append(tool.WorkerTurns, turn)
	idx := len(tool.WorkerTurns) - 1
	p.parse.workerCurrentTurn[wk] = &tool.WorkerTurns[idx]
	return p.parse.workerCurrentTurn[wk]
}

func (p *Projector) activeTurn(msg stream.Message) *TurnView {
	if msg.Scope == stream.ScopeWorker {
		wk := p.workerMapKeyFrom(msg)
		if wk != "" {
			if t, ok := p.parse.workerCurrentTurn[wk]; ok {
				return t
			}
		}
		if t := p.ensureWorkerTurn(msg, 1, ""); t != nil {
			return t
		}
		return nil
	}
	if p.parse.coordinatorTurn == nil {
		return p.ensureCoordinatorTurn(1, "")
	}
	return p.parse.coordinatorTurn
}

func (p *Projector) findToolInTurn(turn *TurnView, toolUseID string) *ToolView {
	for i := range turn.Tools {
		if turn.Tools[i].ToolCallID == toolUseID {
			return &turn.Tools[i]
		}
	}
	return nil
}

func (p *Projector) findTool(toolUseID string) *ToolView {
	tool, _ := p.findToolGlobal(toolUseID)
	return tool
}

// findToolGlobal 在协调者轮次及嵌套 Worker 轮次中定位工具条目。
func (p *Projector) findToolGlobal(toolUseID string) (*ToolView, *TurnView) {
	if toolUseID == "" {
		return nil, nil
	}
	for i := range p.state.Turns {
		coord := &p.state.Turns[i]
		if t := p.findToolInTurn(coord, toolUseID); t != nil {
			return t, coord
		}
		for j := range coord.Tools {
			for k := range coord.Tools[j].WorkerTurns {
				wt := &coord.Tools[j].WorkerTurns[k]
				if t := p.findToolInTurn(wt, toolUseID); t != nil {
					return t, wt
				}
			}
		}
	}
	return nil, nil
}

// resolveToolTurn 优先在已有工具条目所在轮次更新，避免 activeTurn 漂移导致完成态丢失。
func (p *Projector) resolveToolTurn(msg stream.Message, toolUseID string) *TurnView {
	if toolUseID != "" {
		if _, turn := p.findToolGlobal(toolUseID); turn != nil {
			return turn
		}
	}
	return p.activeTurn(msg)
}

func (p *Projector) upsertToolInTurn(turn *TurnView, toolUseID, toolName, status, serverName, message string, elapsed int64) *ToolView {
	if toolUseID == "" {
		return nil
	}
	tool := p.findToolInTurn(turn, toolUseID)
	name := toolName
	if name == "" && tool != nil {
		name = tool.ToolCallName
	}
	if name == "" {
		name = "tool"
	}
	if tool == nil {
		turn.Tools = append(turn.Tools, ToolView{
			ToolCallID: toolUseID, ToolCallName: name, Status: status,
			ServerName: serverName, Preview: message, ElapsedMs: elapsed, LogURL: p.toolLogURL(toolUseID),
		})
		return &turn.Tools[len(turn.Tools)-1]
	}
	tool.Status = status
	tool.ToolCallName = name
	if serverName != "" {
		tool.ServerName = serverName
	}
	if message != "" {
		tool.Preview = message
	}
	if elapsed > 0 {
		tool.ElapsedMs = elapsed
	}
	if tool.LogURL == "" {
		tool.LogURL = p.toolLogURL(toolUseID)
	}
	return tool
}

func (p *Projector) appendToolOutputDelta(turn *TurnView, toolUseID, toolName, delta string) {
	if toolUseID == "" {
		return
	}
	tool := p.findToolInTurn(turn, toolUseID)
	name := toolName
	if name == "" && tool != nil {
		name = tool.ToolCallName
	}
	if name == "" {
		name = "tool"
	}
	if tool == nil {
		turn.Tools = append(turn.Tools, ToolView{
			ToolCallID: toolUseID, ToolCallName: name, Status: "loading",
			LiveOutput: "", OutputStreaming: true, LogURL: p.toolLogURL(toolUseID),
		})
		tool = &turn.Tools[len(turn.Tools)-1]
	}
	tool.Status = "loading"
	tool.OutputStreaming = true
	if toolName != "" {
		tool.ToolCallName = toolName
	}
	if tool.LogURL == "" {
		tool.LogURL = p.toolLogURL(toolUseID)
	}
	tool.LiveOutput += delta
	if len([]rune(tool.LiveOutput)) > maxLiveOutputRunes {
		runes := []rune(tool.LiveOutput)
		tool.LiveOutput = string(runes[len(runes)-maxLiveOutputRunes:])
	}
}

func (p *Projector) resolveParentToolUseID(msg stream.Message) string {
	if msg.ParentToolUseID != nil && *msg.ParentToolUseID != "" {
		return *msg.ParentToolUseID
	}
	if msg.AgentID != "" {
		return p.parse.workerParentToolID[msg.AgentID]
	}
	return ""
}

func (p *Projector) workerMapKeyFrom(msg stream.Message) string {
	parentID := p.resolveParentToolUseID(msg)
	if parentID == "" || msg.AgentID == "" {
		return ""
	}
	return parentID + ":" + msg.AgentID
}

func (p *Projector) rememberCoordinatorToolTurn(msg stream.Message, toolUseID string) {
	if msg.Scope == stream.ScopeWorker || toolUseID == "" || p.parse.coordinatorTurn == nil {
		return
	}
	p.parse.parentToolCoordinatorTurn[toolUseID] = p.parse.coordinatorTurn.Turn
}

// ensureParentAgentToolPlaceholder 在 Worker 事件早于父 agent 工具投影时创建占位条目。
func (p *Projector) ensureParentAgentToolPlaceholder(parentID string) *ToolView {
	if parentID == "" {
		return nil
	}
	if t := p.findTool(parentID); t != nil {
		return t
	}
	turnNum := 1
	if n, ok := p.parse.parentToolCoordinatorTurn[parentID]; ok && n > 0 {
		turnNum = n
	} else if p.parse.coordinatorTurn != nil {
		turnNum = p.parse.coordinatorTurn.Turn
	}
	coord := p.ensureCoordinatorTurn(turnNum, "")
	coord.Tools = append(coord.Tools, ToolView{
		ToolCallID: parentID, ToolCallName: "agent", Status: "loading",
	})
	return &coord.Tools[len(coord.Tools)-1]
}

func isProvisionalToolID(id string) bool {
	return strings.HasPrefix(id, "pending-") || strings.HasPrefix(id, "input-")
}

// reconcileProvisionalTools 在真实 tool_use id 就绪后移除 pending-/input- 占位条目。
func (p *Projector) reconcileProvisionalTools(turn *TurnView, toolName, realID string) {
	if turn == nil || realID == "" || toolName == "" || isProvisionalToolID(realID) {
		return
	}
	var carriedPreview string
	out := turn.Tools[:0]
	for _, t := range turn.Tools {
		if t.ToolCallID == realID {
			out = append(out, t)
			continue
		}
		if isProvisionalToolID(t.ToolCallID) && t.ToolCallName == toolName {
			if carriedPreview == "" && t.Preview != "" {
				carriedPreview = t.Preview
			}
			continue
		}
		out = append(out, t)
	}
	turn.Tools = out
	if carriedPreview == "" {
		return
	}
	if real := p.findToolInTurn(turn, realID); real != nil && real.Preview == "" {
		real.Preview = carriedPreview
	}
}

func mapToolStatus(status string) string {
	switch status {
	case "started", "streaming", "input_streaming":
		return "loading"
	case "failed":
		return "error"
	case "completed", "success":
		return "success"
	default:
		return "loading"
	}
}

func countFinishedTools(tools []ToolView) int {
	n := 0
	for _, t := range tools {
		if t.Status == "success" || t.Status == "error" {
			n++
		}
	}
	return n
}

func (p *Projector) refreshTurnSummary(turn *TurnView) {
	if turn == nil {
		return
	}
	turn.Summary = activity.DeriveTurnSummary(turn.Turn, toolSummaryInputs(turn.Tools), turn.Message, turn.Thinking)
}

func toolSummaryInputs(tools []ToolView) []activity.ToolSummaryInput {
	if len(tools) == 0 {
		return nil
	}
	out := make([]activity.ToolSummaryInput, 0, len(tools))
	for _, t := range tools {
		out = append(out, activity.ToolSummaryInput{
			Name:       t.ToolCallName,
			Preview:    t.Preview,
			LiveOutput: t.LiveOutput,
		})
	}
	return out
}

func subagentStatusLabel(snap agent.Snapshot) string {
	if snap.Description != "" {
		return fmt.Sprintf("子任务：%s…", snap.Description)
	}
	return "子任务进行中…"
}

func (p *Projector) coordTurnNum() int {
	if p.parse.coordinatorTurn != nil {
		return p.parse.coordinatorTurn.Turn
	}
	return 0
}

func toolPreview(t *ToolView) string {
	if t == nil {
		return ""
	}
	return t.Preview
}

func truncateStr(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func cloneState(s RunViewState) RunViewState {
	out := s
	if s.Turns != nil {
		out.Turns = append([]TurnView(nil), s.Turns...)
		for i := range out.Turns {
			if s.Turns[i].Tools != nil {
				out.Turns[i].Tools = append([]ToolView(nil), s.Turns[i].Tools...)
				for j := range out.Turns[i].Tools {
					if s.Turns[i].Tools[j].WorkerTurns != nil {
						out.Turns[i].Tools[j].WorkerTurns = append([]TurnView(nil), s.Turns[i].Tools[j].WorkerTurns...)
					}
				}
			}
		}
	}
	if s.Subagents != nil {
		out.Subagents = make(map[string]SubagentView, len(s.Subagents))
		for k, v := range s.Subagents {
			out.Subagents[k] = v
		}
	}
	if s.Result != nil {
		out.Result = new(*s.Result)
	}
	return out
}

// ExtractReplyText 从视图状态提取面向用户的回复文本。
func ExtractReplyText(state RunViewState) string {
	if state.Result != nil && state.Result.Output != "" && !state.Result.IsError {
		return strings.TrimSpace(state.Result.Output)
	}
	for i := len(state.Turns) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(state.Turns[i].Message); t != "" {
			return t
		}
	}
	return strings.TrimSpace(state.ReplyText)
}
