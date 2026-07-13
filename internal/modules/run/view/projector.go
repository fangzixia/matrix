package view

import (
	"fmt"
	ai "matrix/ai/sdk"
	"matrix/internal/modules/run/view/activity"
	"strings"
	"time"
)

const maxLiveOutputRunes = 8192

// Projector 将 AG-UI 事件投影为 RunViewState。
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

// OnSubagent 由 coordinator 直接更新子 Agent 快照。
func (p *Projector) OnSubagent(snap ai.AgentSnapshot) {
	p.setSubagentFromSnapshot(snap)
	p.state.StatusLabel = subagentStatusLabel(snap)
	p.lastSnap = time.Time{}
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

func (p *Projector) setSubagentFromSnapshot(snap ai.AgentSnapshot) {
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

func subagentStatusLabel(snap ai.AgentSnapshot) string {
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
