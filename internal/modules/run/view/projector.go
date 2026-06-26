package view

import (
	"fmt"
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
	coordinatorTurn   *TurnView
	workerCurrentTurn map[string]*TurnView
}

// NewProjector 创建 Run 视图投影器。
func NewProjector(runID, projectID string) *Projector {
	s := NewRunViewState(runID)
	return &Projector{
		runID:     runID,
		projectID: projectID,
		state:     s,
		parse: parserState{
			workerCurrentTurn: make(map[string]*TurnView),
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
		p.state.StatusLabel = turnStatusLabel(turnNum, data.Transition, data.Summary)
		out = append(out, p.emitActivitySnapshot(now))

	case stream.DataToolOutputDelta:
		p.appendToolOutputDelta(p.activeTurn(msg), msg.ToolUseID, data.ToolName, data.Delta)

	case stream.DataToolProgress, stream.DataMCPProgress:
		if data.Status == "input_streaming" && data.Delta != "" {
			turn := p.activeTurn(msg)
			toolUseID := msg.ToolUseID
			if toolUseID == "" {
				toolUseID = fmt.Sprintf("input-%s", data.ToolName)
			}
			tool := p.upsertToolInTurn(turn, toolUseID, data.ToolName, "loading", data.ServerName, "", 0)
			tool.Preview = tool.Preview + data.Delta
			out = append(out, Envelope{
				Type: EventTOOLCallArgs, RunID: p.runID, Timestamp: now,
				Payload: ToolCallArgsPayload{ToolCallID: toolUseID, Delta: data.Delta},
			})
		} else if data.Status == "started" {
			turn := p.activeTurn(msg)
			toolUseID := msg.ToolUseID
			if toolUseID == "" {
				return out
			}
			p.upsertToolInTurn(turn, toolUseID, data.ToolName, "loading", data.ServerName, data.Message, 0)
			p.state.StatusLabel = fmt.Sprintf("正在执行 %s…", data.ToolName)
			out = append(out, Envelope{
				Type: EventTOOLCallStart, RunID: p.runID, Timestamp: now,
				Payload: ToolCallStartPayload{
					ToolCallID: toolUseID, ToolCallName: data.ToolName, ServerName: data.ServerName,
				},
			})
		} else if data.Status == "completed" || data.Status == "failed" {
			turn := p.activeTurn(msg)
			toolUseID := msg.ToolUseID
			status := mapToolStatus(data.Status)
			tool := p.upsertToolInTurn(turn, toolUseID, data.ToolName, status, data.ServerName, data.Message, data.ElapsedTimeMs)
			tool.OutputStreaming = false
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
		}
	}
	return out
}

func (p *Projector) applyStreamEvent(msg stream.Message, now int64) []Envelope {
	if msg.Event == nil {
		return nil
	}
	turn := p.activeTurn(msg)
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
			out = append(out, Envelope{
				Type: EventREASONINGMessageContent, RunID: p.runID, Timestamp: now,
				Payload: TextDeltaPayload{MessageID: mid, Delta: ev.Delta.Thinking},
			})
		}
		if ev.Delta.Type == stream.DeltaText && ev.Delta.Text != "" {
			turn.Message += ev.Delta.Text
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
	parentID := ""
	if msg.ParentToolUseID != nil {
		parentID = *msg.ParentToolUseID
	}
	tool := p.findTool(parentID)
	if tool == nil {
		return p.ensureCoordinatorTurn(maxInt(p.coordTurnNum(), 1), summary)
	}
	wk := workerMapKey(msg)
	if wk == "" {
		return p.ensureCoordinatorTurn(maxInt(p.coordTurnNum(), 1), summary)
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
		wk := workerMapKey(msg)
		if wk != "" {
			if t, ok := p.parse.workerCurrentTurn[wk]; ok {
				return t
			}
		}
		return p.ensureWorkerTurn(msg, 1, "")
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
	if toolUseID == "" {
		return nil
	}
	for i := range p.state.Turns {
		if t := p.findToolInTurn(&p.state.Turns[i], toolUseID); t != nil {
			return t
		}
	}
	return nil
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
			ServerName: serverName, Preview: message, ElapsedMs: elapsed,
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
			LiveOutput: "", OutputStreaming: true,
		})
		tool = &turn.Tools[len(turn.Tools)-1]
	}
	tool.Status = "loading"
	tool.OutputStreaming = true
	if toolName != "" {
		tool.ToolCallName = toolName
	}
	tool.LiveOutput += delta
	if len([]rune(tool.LiveOutput)) > maxLiveOutputRunes {
		runes := []rune(tool.LiveOutput)
		tool.LiveOutput = string(runes[len(runes)-maxLiveOutputRunes:])
	}
}

func workerMapKey(msg stream.Message) string {
	if msg.ParentToolUseID == nil || msg.AgentID == "" {
		return ""
	}
	return *msg.ParentToolUseID + ":" + msg.AgentID
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

func turnStatusLabel(turn int, transition, summary string) string {
	if summary != "" {
		return summary
	}
	if transition != "" {
		return fmt.Sprintf("第 %d 轮：%s", turn, transition)
	}
	return fmt.Sprintf("第 %d 轮", turn)
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
		r := *s.Result
		out.Result = &r
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
